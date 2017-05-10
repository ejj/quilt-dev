package minion

import (
	"sort"

	"github.com/quilt/quilt/db"
	"github.com/quilt/quilt/join"
	"github.com/quilt/quilt/stitch"

	log "github.com/Sirupsen/logrus"
)

func updatePolicy(view db.Database, blueprint string) {
	compiled, err := stitch.FromJSON(blueprint)
	if err != nil {
		log.WithError(err).Warn("Invalid blueprint.")
		return
	}

	c.Inc("Update Policy")
	updateImages(view, compiled)
	updateContainers(view, compiled)
	updateConnections(view, compiled)
	updatePlacements(view, compiled)
}

// `portPlacements` creates exclusive placement rules such that no two containers
// listening on the same public port get placed on the same machine.
func portPlacements(connections []db.Connection) (placements []db.Placement) {
	ports := make(map[int][]string)
	for _, c := range connections {
		if c.From != stitch.PublicInternetLabel {
			continue
		}

		// XXX: Public connections do not currently support ranges, so we can
		// safely consider just the MinPort.
		ports[c.MinPort] = append(ports[c.MinPort], c.To)
	}

	for _, labels := range ports {
		for _, tgt := range labels {
			for _, other := range labels {
				placements = append(placements,
					db.Placement{
						Exclusive:   true,
						TargetLabel: tgt,
						OtherLabel:  other,
					},
				)
			}
		}
	}

	return placements
}

func updatePlacements(view db.Database, blueprint stitch.Stitch) {
	placements := db.PlacementSlice(portPlacements(view.SelectFromConnection(nil)))
	for _, sp := range blueprint.Placements {
		placements = append(placements, db.Placement{
			TargetLabel: sp.TargetLabel,
			Exclusive:   sp.Exclusive,
			OtherLabel:  sp.OtherLabel,
			Provider:    sp.Provider,
			Size:        sp.Size,
			Region:      sp.Region,
			FloatingIP:  sp.FloatingIP,
		})
	}

	key := func(val interface{}) interface{} {
		p := val.(db.Placement)
		p.ID = 0
		return p
	}

	dbPlacements := db.PlacementSlice(view.SelectFromPlacement(nil))
	_, addSet, removeSet := join.HashJoin(placements, dbPlacements, key, key)

	for _, toAddIntf := range addSet {
		toAdd := toAddIntf.(db.Placement)

		id := view.InsertPlacement().ID
		toAdd.ID = id
		view.Commit(toAdd)
	}

	for _, toRemove := range removeSet {
		view.Remove(toRemove.(db.Placement))
	}
}

func updateConnections(view db.Database, blueprint stitch.Stitch) {
	scs, vcs := stitch.ConnectionSlice(blueprint.Connections),
		view.SelectFromConnection(nil)

	dbcKey := func(val interface{}) interface{} {
		c := val.(db.Connection)
		return stitch.Connection{
			From:    c.From,
			To:      c.To,
			MinPort: c.MinPort,
			MaxPort: c.MaxPort,
		}
	}

	pairs, stitches, dbcs := join.HashJoin(scs, db.ConnectionSlice(vcs), nil, dbcKey)

	for _, dbc := range dbcs {
		view.Remove(dbc.(db.Connection))
	}

	for _, stitchc := range stitches {
		pairs = append(pairs, join.Pair{L: stitchc, R: view.InsertConnection()})
	}

	for _, pair := range pairs {
		stitchc := pair.L.(stitch.Connection)
		dbc := pair.R.(db.Connection)

		dbc.From = stitchc.From
		dbc.To = stitchc.To
		dbc.MinPort = stitchc.MinPort
		dbc.MaxPort = stitchc.MaxPort
		view.Commit(dbc)
	}
}

func queryContainers(blueprint stitch.Stitch) []db.Container {
	containers := map[string]*db.Container{}
	for _, c := range blueprint.Containers {
		containers[c.ID] = &db.Container{
			StitchID:          c.ID,
			Command:           c.Command,
			Env:               c.Env,
			FilepathToContent: c.FilepathToContent,
			Image:             c.Image.Name,
			Dockerfile:        c.Image.Dockerfile,
			Hostname:          c.Hostname,
		}
	}

	for _, label := range blueprint.Labels {
		for _, id := range label.IDs {
			containers[id].Labels = append(containers[id].Labels, label.Name)
		}
	}

	var ret []db.Container
	for _, c := range containers {
		ret = append(ret, *c)
	}

	return ret
}

func updateContainers(view db.Database, blueprint stitch.Stitch) {
	key := func(val interface{}) interface{} {
		return val.(db.Container).StitchID
	}

	pairs, news, dbcs := join.HashJoin(db.ContainerSlice(queryContainers(blueprint)),
		db.ContainerSlice(view.SelectFromContainer(nil)), key, key)

	for _, dbc := range dbcs {
		view.Remove(dbc.(db.Container))
	}

	for _, new := range news {
		pairs = append(pairs, join.Pair{L: new, R: view.InsertContainer()})
	}

	for _, pair := range pairs {
		newc := pair.L.(db.Container)
		dbc := pair.R.(db.Container)

		// By sorting the labels we prevent the database from getting confused
		// when their order is non deterministic.
		dbc.Labels = newc.Labels
		sort.Sort(sort.StringSlice(dbc.Labels))

		dbc.Command = newc.Command
		dbc.Image = newc.Image
		dbc.Dockerfile = newc.Dockerfile
		dbc.Env = newc.Env
		dbc.FilepathToContent = newc.FilepathToContent
		dbc.StitchID = newc.StitchID
		dbc.Hostname = newc.Hostname
		view.Commit(dbc)
	}
}

func updateImages(view db.Database, blueprint stitch.Stitch) {
	dbImageKey := func(intf interface{}) interface{} {
		return stitch.Image{
			Name:       intf.(db.Image).Name,
			Dockerfile: intf.(db.Image).Dockerfile,
		}
	}

	blueprintImages := stitchImageSlice(queryImages(blueprint))
	dbImages := db.ImageSlice(view.SelectFromImage(nil))
	_, toAdd, toRemove := join.HashJoin(blueprintImages, dbImages, nil, dbImageKey)

	for _, intf := range toAdd {
		im := view.InsertImage()
		im.Name = intf.(stitch.Image).Name
		im.Dockerfile = intf.(stitch.Image).Dockerfile
		view.Commit(im)
	}

	for _, row := range toRemove {
		view.Remove(row.(db.Image))
	}
}

func queryImages(blueprint stitch.Stitch) (images []stitch.Image) {
	addedImages := map[stitch.Image]struct{}{}
	for _, c := range blueprint.Containers {
		_, addedImage := addedImages[c.Image]
		if c.Image.Dockerfile == "" || addedImage {
			continue
		}

		images = append(images, c.Image)
		addedImages[c.Image] = struct{}{}
	}
	return images
}

type stitchImageSlice []stitch.Image

func (slc stitchImageSlice) Get(ii int) interface{} {
	return slc[ii]
}

func (slc stitchImageSlice) Len() int {
	return len(slc)
}

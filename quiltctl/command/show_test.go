package command

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	units "github.com/docker/go-units"
	"github.com/stretchr/testify/assert"

	"github.com/quilt/quilt/api/client/mocks"
	"github.com/quilt/quilt/db"
)

func TestShowFlags(t *testing.T) {
	t.Parallel()

	expHost := "IP"

	cmd := NewShowCommand()
	err := parseHelper(cmd, []string{"-H", expHost})

	assert.NoError(t, err)
	assert.Equal(t, expHost, cmd.host)

	cmd = NewShowCommand()
	err = parseHelper(cmd, []string{"-no-trunc"})

	assert.NoError(t, err)
	assert.True(t, cmd.noTruncate)
}

func TestShowErrors(t *testing.T) {
	t.Parallel()

	mockErr := errors.New("error")

	// Error querying containers
	mockClient := new(mocks.Client)
	mockClient.On("QueryConnections").Return(nil, nil)
	mockClient.On("QueryMachines").Return([]db.Machine{{Status: db.Connected}}, nil)
	mockClient.On("QueryContainers").Return(nil, mockErr)
	mockClient.On("QueryImages").Return(nil, nil)
	cmd := &Show{false, connectionHelper{client: mockClient}}
	assert.EqualError(t, cmd.run(), "unable to query containers: error")

	// Error querying connections from LeaderClient
	mockClient = new(mocks.Client)
	mockClient.On("QueryContainers").Return(nil, nil)
	mockClient.On("QueryMachines").Return([]db.Machine{{Status: db.Connected}}, nil)
	mockClient.On("QueryConnections").Return(nil, mockErr)
	mockClient.On("QueryImages").Return(nil, nil)
	cmd = &Show{false, connectionHelper{client: mockClient}}
	assert.EqualError(t, cmd.run(), "unable to query connections: error")
}

// Test that we don't query the cluster if it's not up.
func TestMachineOnly(t *testing.T) {
	t.Parallel()

	mockClient := new(mocks.Client)
	cmd := &Show{false, connectionHelper{client: mockClient}}

	// Test failing to query machines.
	mockClient.On("QueryMachines").Once().Return(nil, assert.AnError)
	cmd.run()
	mockClient.AssertNotCalled(t, "QueryContainers")

	// Test no machines in database.
	mockClient.On("QueryMachines").Once().Return(nil, nil)
	cmd.run()
	mockClient.AssertNotCalled(t, "QueryContainers")

	// Test no connected machines.
	mockClient.On("QueryMachines").Once().Return(
		[]db.Machine{{Status: db.Booting}}, nil)
	cmd.run()
	mockClient.AssertNotCalled(t, "QueryContainers")
}

func TestShowSuccess(t *testing.T) {
	t.Parallel()

	mockClient := new(mocks.Client)
	mockClient.On("QueryContainers").Return(nil, nil)
	mockClient.On("QueryMachines").Return(nil, nil)
	mockClient.On("QueryConnections").Return(nil, nil)
	mockClient.On("QueryImages").Return(nil, nil)
	cmd := &Show{false, connectionHelper{client: mockClient}}
	assert.Equal(t, 0, cmd.Run())
}

func TestMachineOutput(t *testing.T) {
	t.Parallel()

	machines := []db.Machine{
		{
			StitchID: "1",
			Role:     db.Master,
			Provider: "Amazon",
			Region:   "us-west-1",
			Size:     "m4.large",
			PublicIP: "8.8.8.8",
			Status:   db.Connected,
		}, {
			StitchID:   "2",
			Role:       db.Worker,
			Provider:   "DigitalOcean",
			Region:     "sfo1",
			Size:       "2gb",
			PublicIP:   "9.9.9.9",
			FloatingIP: "10.10.10.10",
			Status:     db.Connected,
		},
	}

	var b bytes.Buffer
	writeMachines(&b, machines)
	result := string(b.Bytes())

	/* By replacing space with underscore, we make the spaces explicit and whitespace
	* errors easier to debug. */
	result = strings.Replace(result, " ", "_", -1)

	exp := `MACHINE____ROLE______PROVIDER________REGION_______SIZE` +
		`________PUBLIC_IP______STATUS
1__________Master____Amazon__________us-west-1____m4.large____8.8.8.8________connected
2__________Worker____DigitalOcean____sfo1_________2gb_________10.10.10.10____connected
`

	assert.Equal(t, exp, result)
}

func checkContainerOutput(t *testing.T, containers []db.Container,
	machines []db.Machine, connections []db.Connection, images []db.Image,
	truncate bool, exp string) {

	var b bytes.Buffer
	writeContainers(&b, containers, machines, connections, images, truncate)

	/* By replacing space with underscore, we make the spaces explicit and whitespace
	* errors easier to debug. */
	result := strings.Replace(b.String(), " ", "_", -1)
	assert.Equal(t, exp, result)
}

func TestContainerOutput(t *testing.T) {
	t.Parallel()

	containers := []db.Container{
		{ID: 1, StitchID: "3", Minion: "3.3.3.3", IP: "1.2.3.4",
			Image: "image1", Command: []string{"cmd", "1"},
			Status: "running"},
		{ID: 2, StitchID: "1", Minion: "1.1.1.1", Image: "image2",
			Labels: []string{"label1", "label2"}, Status: "scheduled"},
		{ID: 3, StitchID: "4", Minion: "1.1.1.1", Image: "image3",
			Command: []string{"cmd"},
			Labels:  []string{"label1"},
			Status:  "scheduled"},
		{ID: 4, StitchID: "7", Minion: "2.2.2.2", Image: "image1",
			Command: []string{"cmd", "3", "4"},
			Labels:  []string{"label1"}},
		{ID: 5, StitchID: "8", Image: "image1"},
	}
	machines := []db.Machine{
		{StitchID: "5", PublicIP: "7.7.7.7", PrivateIP: "1.1.1.1"},
		{StitchID: "6", PrivateIP: "2.2.2.2"},
		{StitchID: "7", PrivateIP: ""},
	}
	connections := []db.Connection{
		{ID: 1, From: "public", To: "label1", MinPort: 80, MaxPort: 80},
		{ID: 2, From: "notpublic", To: "label2", MinPort: 100, MaxPort: 101},
	}

	expected := `CONTAINER____MACHINE____COMMAND___________LABELS________` +
		`____STATUS_______CREATED____PUBLIC_IP
3_______________________image1_cmd_1________________________running_________________
____________________________________________________________________________________
1____________5__________image2____________label1,_label2____scheduled__________` +
		`_____7.7.7.7:80
4____________5__________image3_cmd________label1____________scheduled__________` +
		`_____7.7.7.7:80
____________________________________________________________________________________
7____________6__________image1_cmd_3_4____label1____________scheduled_______________
____________________________________________________________________________________
8____________7__________image1______________________________________________________
`
	checkContainerOutput(t, containers, machines, connections, nil, true, expected)

	// Testing writeContainers with created time values.
	mockTime := time.Now()
	humanDuration := units.HumanDuration(time.Since(mockTime))
	mockCreatedString := fmt.Sprintf("%s ago", humanDuration)
	mockCreatedString = strings.Replace(mockCreatedString, " ", "_", -1)

	containers = []db.Container{
		{ID: 1, StitchID: "3", Minion: "3.3.3.3", IP: "1.2.3.4",
			Image: "image1", Command: []string{"cmd", "1"},
			Status: "running", Created: mockTime.UTC()},
	}
	machines = []db.Machine{}
	connections = []db.Connection{}

	expected = `CONTAINER____MACHINE____COMMAND_________LABELS____STATUS___` +
		`__CREATED___________________PUBLIC_IP
3_______________________image1_cmd_1______________running____` + mockCreatedString +
		`____
`
	checkContainerOutput(t, containers, machines, connections, nil, true, expected)

	// Testing writeContainers with longer durations.
	mockDuration := time.Hour
	mockTime = time.Now().Add(-mockDuration)
	humanDuration = units.HumanDuration(time.Since(mockTime))
	mockCreatedString = fmt.Sprintf("%s ago", humanDuration)
	mockCreatedString = strings.Replace(mockCreatedString, " ", "_", -1)

	containers = []db.Container{
		{ID: 1, StitchID: "3", Minion: "3.3.3.3", IP: "1.2.3.4",
			Image: "image1", Command: []string{"cmd", "1"},
			Status: "running", Created: mockTime.UTC()},
	}
	machines = []db.Machine{}
	connections = []db.Connection{}

	expected = `CONTAINER____MACHINE____COMMAND_________LABELS____STATUS___` +
		`__CREATED______________PUBLIC_IP
3_______________________image1_cmd_1______________running____` + mockCreatedString +
		`____
`
	checkContainerOutput(t, containers, machines, connections, nil, true, expected)

	// Test that long outputs are truncated when `truncate` is true
	containers = []db.Container{
		{ID: 1, StitchID: "3", Minion: "3.3.3.3", IP: "1.2.3.4",
			Image: "image1", Command: []string{"cmd", "1", "&&", "cmd",
				"91283403472903847293014320984723908473248-23843984"},
			Status: "running", Created: mockTime.UTC()},
	}
	machines = []db.Machine{}
	connections = []db.Connection{}

	expected = `CONTAINER____MACHINE____COMMAND_____________________________` +
		`_LABELS____STATUS_____CREATED______________PUBLIC_IP
3_______________________image1_cmd_1_&&_cmd_9128340347...______________running____` +
		mockCreatedString + `____
`
	checkContainerOutput(t, containers, machines, connections, nil, true, expected)

	// Test that long outputs are not truncated when `truncate` is false
	expected = `CONTAINER____MACHINE____COMMAND___________________________________` +
		`________________________________LABELS____STATUS_____CREATED_________` +
		`_____PUBLIC_IP
3_______________________image1_cmd_1_&&_cmd_91283403472903847293014320984723908473248` +
		`-23843984______________running____` + mockCreatedString + `____
`
	checkContainerOutput(t, containers, machines, connections, nil, false, expected)

	// Test writing container that has multiple labels connected to the public
	// internet.
	containers = []db.Container{
		{StitchID: "3", Minion: "1.1.1.1", Image: "image1",
			Labels: []string{"red"}},
	}
	machines = []db.Machine{
		{StitchID: "5", PublicIP: "7.7.7.7", PrivateIP: "1.1.1.1"},
	}
	connections = []db.Connection{
		{ID: 1, From: "public", To: "red", MinPort: 80, MaxPort: 80},
		{ID: 2, From: "public", To: "red", MinPort: 100, MaxPort: 101},
	}

	expected = `CONTAINER____MACHINE____COMMAND____LABELS____STATUS` +
		`_______CREATED____PUBLIC_IP
3____________5__________image1_____red_______scheduled` +
		`_______________7.7.7.7:[80,100-101]
`
	checkContainerOutput(t, containers, machines, connections, nil, true, expected)
}

func TestContainerOutputCustomImage(t *testing.T) {
	t.Parallel()

	// Building.
	containers := []db.Container{
		{StitchID: "3", Image: "custom-dockerfile"},
	}
	images := []db.Image{
		{Name: "custom-dockerfile", Status: db.Building},
	}

	exp := `CONTAINER____MACHINE____COMMAND_______________LABELS____` +
		`STATUS______CREATED____PUBLIC_IP
3_______________________custom-dockerfile_______________building_______________
`
	checkContainerOutput(t, containers, nil, nil, images, true, exp)

	// Built, but not scheduled.
	images = []db.Image{
		{Name: "custom-dockerfile", Status: db.Built},
	}
	exp = `CONTAINER____MACHINE____COMMAND_______________LABELS____` +
		`STATUS____CREATED____PUBLIC_IP
3_______________________custom-dockerfile_______________built________________
`
	checkContainerOutput(t, containers, nil, nil, images, true, exp)

	// We only have image data on a different image, so we can't update the status.
	images = []db.Image{
		{Name: "ignoreme", Status: db.Built},
	}
	exp = `CONTAINER____MACHINE____COMMAND_______________LABELS____` +
		`STATUS____CREATED____PUBLIC_IP
3_______________________custom-dockerfile____________________________________
`
	checkContainerOutput(t, containers, nil, nil, images, true, exp)

	// Built and scheduled.
	images = []db.Image{
		{Name: "custom-dockerfile", Status: db.Built},
	}
	containers = []db.Container{
		{StitchID: "3", Image: "custom-dockerfile", Minion: "foo"},
	}
	exp = `CONTAINER____MACHINE____COMMAND_______________LABELS` +
		`____STATUS_______CREATED____PUBLIC_IP
3_______________________custom-dockerfile_______________scheduled_______________
`
	checkContainerOutput(t, containers, nil, nil, images, true, exp)
}

func TestContainerStr(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", containerStr("", nil, false))
	assert.Equal(t, "", containerStr("", []string{"arg0"}, false))
	assert.Equal(t, "container arg0 arg1",
		containerStr("container", []string{"arg0", "arg1"}, false))
}

func TestPublicIPStr(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", publicIPStr(db.Machine{}, nil))
	assert.Equal(t, "", publicIPStr(db.Machine{}, []string{"80-88"}))
	assert.Equal(t, "", publicIPStr(db.Machine{PublicIP: "1.2.3.4"}, nil))
	assert.Equal(t, "1.2.3.4:80-88",
		publicIPStr(db.Machine{PublicIP: "1.2.3.4"}, []string{"80-88"}))
	assert.Equal(t, "1.2.3.4:[70,80-88]",
		publicIPStr(db.Machine{PublicIP: "1.2.3.4"}, []string{"70", "80-88"}))
	assert.Equal(t, "8.8.8.8:[70,80-88]",
		publicIPStr(db.Machine{PublicIP: "1.2.3.4", FloatingIP: "8.8.8.8"},
			[]string{"70", "80-88"}))
}

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

func TestPsFlags(t *testing.T) {
	t.Parallel()

	expHost := "IP"

	cmd := NewPsCommand()
	err := parseHelper(cmd, []string{"-H", expHost})

	assert.NoError(t, err)
	assert.Equal(t, expHost, cmd.host)

	cmd = NewPsCommand()
	err = parseHelper(cmd, []string{"-no-trunc"})

	assert.NoError(t, err)
	assert.True(t, cmd.noTruncate)
}

func TestPsErrors(t *testing.T) {
	t.Parallel()

	mockErr := errors.New("error")

	// Error querying containers
	mockClient := &mocks.Client{ContainerErr: mockErr}
	cmd := &Ps{false, connectionHelper{client: mockClient}}
	assert.EqualError(t, cmd.run(), "unable to query containers: error")

	// Error querying connections from LeaderClient
	mockClient = &mocks.Client{ConnectionErr: mockErr}
	cmd = &Ps{false, connectionHelper{client: mockClient}}
	assert.EqualError(t, cmd.run(), "unable to query connections: error")
}

func TestPsSuccess(t *testing.T) {
	t.Parallel()

	mockClient := new(mocks.Client)
	cmd := &Ps{false, connectionHelper{client: mockClient}}
	assert.Equal(t, 0, cmd.Run())
}

func TestMachineOutput(t *testing.T) {
	t.Parallel()

	machines := []db.Machine{{
		StitchID: "1",
		Role:     db.Master,
		Provider: "Amazon",
		Region:   "us-west-1",
		Size:     "m4.large",
		PublicIP: "8.8.8.8",
	}}

	var b bytes.Buffer
	writeMachines(&b, machines)
	result := string(b.Bytes())

	/* By replacing space with underscore, we make the spaces explicit and whitespace
	* errors easier to debug. */
	result = strings.Replace(result, " ", "_", -1)

	exp := `MACHINE____ROLE______PROVIDER____REGION_______SIZE` +
		`________PUBLIC_IP____STATUS
1__________Master____Amazon______us-west-1____m4.large____8.8.8.8______disconnected
`

	assert.Equal(t, exp, result)
}

func checkContainerOutput(t *testing.T, containers []db.Container,
	machines []db.Machine, connections []db.Connection, truncate bool, exp string) {

	var b bytes.Buffer
	writeContainers(&b, containers, machines, connections, truncate)

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
	checkContainerOutput(t, containers, machines, connections, true, expected)

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
	checkContainerOutput(t, containers, machines, connections, true, expected)

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
	checkContainerOutput(t, containers, machines, connections, true, expected)

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
	checkContainerOutput(t, containers, machines, connections, true, expected)

	// Test that long outputs are not truncated when `truncate` is false
	expected = `CONTAINER____MACHINE____COMMAND___________________________________` +
		`________________________________LABELS____STATUS_____CREATED_________` +
		`_____PUBLIC_IP
3_______________________image1_cmd_1_&&_cmd_91283403472903847293014320984723908473248` +
		`-23843984______________running____` + mockCreatedString + `____
`
	checkContainerOutput(t, containers, machines, connections, false, expected)

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
	checkContainerOutput(t, containers, machines, connections, true, expected)
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
	assert.Equal(t, "", publicIPStr("", nil))
	assert.Equal(t, "", publicIPStr("", []string{"80-88"}))
	assert.Equal(t, "", publicIPStr("1.2.3.4", nil))
	assert.Equal(t, "1.2.3.4:80-88", publicIPStr("1.2.3.4", []string{"80-88"}))
	assert.Equal(t, "1.2.3.4:[70,80-88]",
		publicIPStr("1.2.3.4", []string{"70", "80-88"}))
}

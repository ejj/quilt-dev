// +build !windows

package cli

import "github.com/kelda/kelda/cli/command"

func init() {
	commands["minion"] = command.NewMinionCommand()
}

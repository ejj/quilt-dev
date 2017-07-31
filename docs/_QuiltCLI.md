# Quilt CLI

Quilt's CLI, `quilt`, is a handy command line tool for starting, stopping, and
managing deployments. Quilt CLI commands have the following format:

```console
$ quilt [OPTIONS] COMMAND
```

To see the help text for a specific command, run:

```console
$ quilt COMMAND --help
```


## Options
| Name, shorthand     | Default | Description                                               |
|---------------------|---------|-----------------------------------------------------------|
| `--log-level`, `-l` | `info`  | Logging level (debug, info, warn, error, fatal, or panic) |
| `--verbose`, `-v`   | `false` | Turn on debug logging                                     |
| `-log-file`         |         | Log output file (will be overwritten)                     |


## Commands

| Name         | Description                                                                                      |
|--------------|--------------------------------------------------------------------------------------------------|
| `counters`   | Display internal counters tracked for debugging purposes. Most users will not need this command. |
| `daemon`     | Start the quilt daemon, which listens for quilt API requests.                                    |
| `debug-logs` | Fetch logs for a set of machines or containers.                                                  |
| `inspect`    | Visualize a blueprint.                                                                           |
| `logs`       | Fetch the logs of a container or machine minion.                                                 |
| `minion`     | Run the quilt minion.                                                                            |
| `show`       | Display the status of quilt-managed machines and containers.                                     |
| `run`        | Compile a blueprint, and deploy the system it describes.                                         |
| `ssh`        | SSH into or execute a command in a machine or container.                                         |
| `stop`       | Stop a deployment.                                                                               |
| `version`    | Show the Quilt version information.                                                              |
| `setup-tls`  | Create the files necessary for TLS-encrypted communication with Quilt.                           |


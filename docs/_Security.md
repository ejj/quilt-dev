# Security

## TLS
Quilt uses [grpc](http://www.grpc.io/) for communication with the daemon and
deployed clusters. Functionality exposed through `grpc` include deploying new
Stitches and querying deployment information. Thus, TLS should be enabled for
all non-experimental deployments. It is currently disabled by default.

### Quickstart
Generate the necessary TLS files.

```console
$ quilt setup-tls ~/.quilt/tls
```

Start the daemon with TLS enabled.

```console
$ quilt daemon -tls-dir ~/.quilt/tls
```

Use the other Quilt commands as normal.

```console
$ quilt run ./example.js
$ quilt show
MACHINE         ROLE      PROVIDER    REGION       SIZE         PUBLIC IP         STATUS
8a0d2198229c    Master    Amazon      us-west-1    m3.medium    54.153.11.92      connected
b92d625c6847    Worker    Amazon      us-west-1    m3.medium    52.53.170.129     connected

CONTAINER       MACHINE         COMMAND                     LABELS    STATUS     CREATED           PUBLIC IP
1daa461f0805    b92d625c6847    alpine tail -f /dev/null    alpine    running    24 seconds ago    52.53.170.129:8000
```

However, trying to connect to a cluster with different credentials fails.
This can be simulated by restarting the daemon and running the same blueprint,
but with different TLS credentials. Note that the machines never connect.

```console
$ quilt daemon -tls-dir ~/.quilt/other-credentials
$ quilt run ./example.js
$ quilt show
MACHINE         ROLE      PROVIDER    REGION       SIZE         PUBLIC IP        STATUS
8a0d2198229c    Master    Amazon      us-west-1    m3.medium    54.153.11.92     connecting
b92d625c6847    Worker    Amazon      us-west-1    m3.medium    52.53.170.129    connecting
```

Trying to connect in Insecure mode also fails. Note that the machines never connect.

```console
$ quilt daemon
$ quilt run ./example.js
$ quilt show
MACHINE         ROLE      PROVIDER    REGION       SIZE         PUBLIC IP        STATUS
8a0d2198229c    Master    Amazon      us-west-1    m3.medium    52.53.170.129    connecting
b92d625c6847    Worker    Amazon      us-west-1    m3.medium    54.153.11.92     connecting
```

### Setup
The certificate hierarchy can be easily created using the `setup-tls` subcommand.
For example,

```console
$ quilt setup-tls ~/.quilt/tls
```

Will create the file structure described in [tls-dir](#tls-dir). No additional
setup is necessary -- the `-tls-dir` flag can now be set to your chosen TLS
directory.

### tls-dir
TLS is enabled with the `-tls-dir` option. The TLS directory should have the
following structure when passed to `quilt daemon`:

```console
$ tree ~/.quilt/tls
├── certificate_authority.crt
├── certificate_authority.key
├── quilt.crt
├── quilt.key
```

- certificate_authority.crt: The certificate authority certificate.
- certificate_authority.crt: The private key of the certificate authority.
Used by the daemon to generate minion certificates.
- quilt.crt: A certificate signed by the certificate authority.
Used for connecting to the cluster.
- quilt.key: The private key associated with the signed certificate.
Used for connecting to the cluster.

Other files in the directory are ignored by Quilt.

### admin-ssh-private-key
The daemon installs keys using SFTP, so the daemon requires SSH access to the
machines. By default, the daemon generates an in-memory key to use for distributing
keys. A key can be specified from the filesystem using the
`admin-ssh-private-key` flag.

For example,

```console
$ quilt daemon -admin-ssh-private-key ~/.quilt/id_rsa -tls-dir ~/.quilt/tls
```

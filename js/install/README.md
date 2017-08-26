This repo contains a Node.js package for easily installing
[Quilt](http://quilt.io) using npm.

To pull the compiled binaries off GitHub, and install them:
```
$ npm install -g @quilt/install
```
Note that the `-g` flag is necessary to install the binaries globally (i.e. so
that `quilt` can be invoked from anywhere).

If installing as root, the `--unsafe-perm` flag is required:
```console
$ sudo npm install -g @quilt/install --unsafe-perm
```

[![Build Status](https://travis-ci.org/quilt/quilt.svg?branch=master)](https://travis-ci.org/quilt/quilt)
[![Go Report Card](https://goreportcard.com/badge/github.com/quilt/quilt)](https://goreportcard.com/report/github.com/quilt/quilt)
[![Code Coverage](https://codecov.io/gh/quilt/quilt/branch/master/graph/badge.svg)](https://codecov.io/gh/quilt/quilt)

# Quilt

Deploying applications to the cloud can be painful. Booting virtual machines, configuring
networks, and setting up databases, requires massive amounts of specialized knowledge —
knowledge that’s scattered across documentation, blog posts, tutorials, and source code.

Quilt aims to make sharing this knowledge simple by encoding it in JavaScript.  Just as
developers package, share, and reuse application code, Quilt’s JavaScript framework makes
it possible to package, share, and reuse the knowledge necessary to run applications in
the cloud.

To take this knowledge into production, simply `quilt run` the JavaScript blueprint of
your application. Quilt will set up virtual machines, configure a secure network, install
containers, and whatever else is needed to get up and running smoothly on your favorite
cloud provider.

## Deploy Quickly on...

![providers](./docs/source/images/providers.png)

## Install

Install Quilt with npm:

```bash
$ npm install -g @quilt/install
```
Check out more in our [Getting Started Guide](http://docs.quilt.io/#getting-started).

## API

Run any container.

[//]: # (b1)
<!-- const {Container, Service, Machine, publicInternet} = require('@quilt/quilt'); -->
```javascript
let webContainer = new Container('someNodejsImage');
```

Load balance traffic with Services.

[//]: # (b1)
```javascript
let webApp = new Service('web', webContainer.replicate(3)); // A load balancer over 3 containers.
```

Share and import blueprints via npm.

[//]: # (b1)
```javascript
const Redis = require('@quilt/redis');
let redis = new Redis(2, 'AUTH_PASSWORD'); // 2 Redis database replicas.
```

Set up a secure network.

[//]: # (b1)
```javascript
webApp.allowFrom(publicInternet, 80); // Open the webservers' port 80 to the public internet.
redis.allowFrom(webApp, redis.port); // Let the web app communicate with Redis.
```

Deploy VMs on any [supported cloud provider](#deploy-quickly-on).

[//]: # (b1)
```javascript
let vm = new Machine({
  provider: 'Amazon',
  size: 't2.micro'
});
```

See [full example blueprints](https://github.com/quilt/) and [check out our docs](http://docs.quilt.io).

## Quilt CLI

```bash
# Deploy your application.
$ quilt run ./someBlueprint.js

# SSH into VMs and containers.
$ quilt ssh <ID>

# Check the status of your deployment.
$ quilt show
```

This is just a small sample of the Quilt CLI. [Check out more handy commands](http://docs.quilt.io/#quilt-cli) for managing your deployment.

## Get Started

* Get started with [our **tutorial**](http://docs.quilt.io/#getting-started)
* Check out [our **docs**](http://docs.quilt.io/)
* [**Contribute** to the project](http://docs.quilt.io/#developing-quilt)
* Learn more on our [**website**](http://quilt.io)
* [**Get in touch!**](http://quilt.io/#contact)

We would love to hear if you have any questions, suggestions, or other comments!

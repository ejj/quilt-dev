# Getting Started

## How Quilt Works

This section describes what happens when you run an application using Quilt;
feel free to skip this section and head straight to [Installing
Quilt](#installing-quilt) if you'd like to quickly get up and running with
Quilt.

The key idea behind Quilt is a blueprint: a blueprint describes every aspect of
running a particular application in the cloud, and is written in JavaScript.
Quilt blueprints exist for many common applications.  Using Quilt, you can run
one of those applications by executing just two commands on your laptop:

![Quilt Diagram](Quilt_Diagram.png)

The first command,`quilt daemon`, starts a long-running process that handles
launching machines in the cloud (on your choice of cloud provider), configuring
those machines, and managing them (e.g., detecting when they've failed so need
to be re-started).  The `quilt daemon` command starts the daemon, but doesn't
yet launch any machines. To launch an application, call `quilt run` with a
JavaScript blueprint (in this example, the blueprint is called `my_app.js`).
The `run` command passes the parsed blueprint to the daemon, and the daemon
sets up the infrastructure described in the blueprint.

Quilt runs applications using Docker containers. You can think of a container
as being like a process: as a coarse rule-of-thumb, anything that you'd launch
as its own process should have it's own container with Quilt.  While containers 
are lightweight (like processes), they each have their own environment
(including their own filesystem and their own software installed) and are
isolated from other containers running on the same machine (unlike processes). 
If you've never used containers before, it may be helpful to review the
[Docker getting started guide](https://docs.docker.com/get-started).

In this example, `my_app.js` described an application made up of three
containers, and it described a cluster with one master machine and two worker
machines.  The master is responsible for managing the worker machines, and no
application containers run on the master.  The application containers are run on
the workers; in this case, Quilt ran two containers on one worker machine and
one container on the other.

## Installing Quilt

Quilt relies on Node.js.  Start by downloading Node.js from the [Node.js
download page](https://nodejs.org/en/download/).  We have only tested Quilt with
Node version v7.10.0 and above.

Next, use Node.js's package manager, `npm`, to install Quilt:

```console
$ npm install -g @quilt/install
```

Note that if installing as root, the `--unsafe-perm` flag is required:

```console
$ sudo npm install -g @quilt/install --unsafe-perm
```

To check that this worked, try launching the Quilt daemon.  This is a
long-running process, so it will not return (you'll need to use a new shell
window to edit and run blueprints).

```console
$ quilt daemon
```

## Configure A Cloud Provider

In order to run any applications with Quilt, you'll need to setup a cloud
provider that Quilt will use to launch machines.  Quilt currently supports
Amazon EC2, Digital Ocean, and Google Compute Engine; support for running
locally with Vagrant is currently experimental.  Contact us if you're interested
in a cloud provider that we don't yet support.

Below, we describe how to setup Amazon EC2; refer to the
[Cloud Providers](#cloud-provider-configuration) section if you'd like to setup
a different cloud provider.

### Amazon EC2

For Amazon EC2, you'll first need to create an account with [Amazon Web
Services](https://aws.amazon.com/ec2/) and then find your access credentials
from the
[Security Credentials](https://console.aws.amazon.com/iam/home?#security_credential)
page in the AWS Management Console.
Once you've done that, put your Amazon credentials in a file called
`~/.aws/credentials`:

```conf
[default]
aws_access_key_id = <YOUR_ID>
aws_secret_access_key = <YOUR_SECRET_KEY>
```

The file needs to appear exactly as above (including the `[default]` at the
top), except with `<YOUR_ID>` and `<YOUR_SECRET_KEY>` filled in appropriately.

## Running Your First Quilt Blueprint

This section will walk you through using Quilt to run Nginx, which is an
open-source HTTP server that.  In the example, we'll use Nginx to serve a
simple webpage. Start by downloading the blueprint using git:

```console
$ git clone https://github.com/quilt/nginx.git
```

The blueprint is the `main.js` file in the nginx directory; take a look at this
file if you'd like an to see an example of what blueprints look like.  This
blueprint will start one master and one worker machine on Amazon AWS, using
t2.micro instances (which are in Amazon's
[free tier](https://aws.amazon.com/free/), meaning that you can run them for
a few hours for free if you're a new Amazon user).  Recall from [How Quilt Works](#how-quilt-works) that the
master is responsible for managing the worker machines, and worker machines are
used to run application containers.  In this case, the worker machine will
serve the webpage in `index.html`.

Note that if you decided
above to setup a different cloud provider, you'll need to update the `Machine`
in `main.js` to use the corresponding cloud provider (e.g., by changing
`"Amazon"` to `"Google"`).

Before running anything, you'll need to download the JavaScript dependencies of
the blueprint.  The Nginx blueprint depends on the `@quilt/quilt` module; more
complicated blueprints may have other dependencies that need to be installed.
Use `npm`, the Node.js package manager, to install all dependencies in the
`nginx` folder:

```console
$ npm install .
```

To run a blueprint, you first need to have a Quilt daemon running.  If you
haven't already started one, open a new terminal window and start it:

```console
$ quilt daemon
```

The daemon is a long running process that periodically prints some log messages.
Leave this running, and use a new terminal window to run the blueprint:

```console
$ quilt run ./main.js
```

This command tells the daemon to launch the machines and containers described in
`main.js`.  It will return immediately, but if you return to the `daemon`
window, you'll see some things starting to happen.  The best way to see what's
happening is to return to the window where you typed `quilt run`, and now
use Quilt's `show` command to list everything that's running:

```console
$ quilt show
MACHINE         ROLE      PROVIDER    REGION       SIZE     PUBLIC IP    STATUS e5b1839d2bea    Master    Amazon      us-west-1    t2.micro              disconnected
e2401c348c78    Worker    Amazon      us-west-1    t2.micro              disconnected
```

Your output will look similar to the output above.  This output means that Quilt
has launched two machines, one as a master and one as a worker, in Amazon.  Both
machines are disconnected, because they're still being initialized. When a
machine is fully booted and configured, it will be marked as connected.
Launching machines on AWS takes a few minutes, and eventually the output of
`show` will look like:

```console
$ quilt show
MACHINE         ROLE      PROVIDER    REGION       SIZE        PUBLIC IP         STATUS
e5b1839d2bea    Master    Amazon      us-west-1    t2.micro    54.183.98.15      connected
e2401c348c78    Worker    Amazon      us-west-1    t2.micro    54.241.251.192    connected

CONTAINER       MACHINE         COMMAND        LABELS           STATUS     CREATED               PUBLIC IP
bd681b3d3af7    e2401c348c78    nginx:1.13     nginx_example    running    About a minute ago    54.241.251.192:80
```

The bottom row lists the container that's running `nginx`.  The `nginx`
deployment is relatively simple and has just one container, but a typical
application running in Quilt will have many containers running (one for each
part of the application; for example, your website application might require a
second container that runs a database).  The last column in that row,
`PUBLIC IP`, says the address you can use to access your website.

By default, Quilt-managed containers are disconnected from the public internet
and isolated from one another. This helps to keep your application secure by
preventing all access except for what you explicitly specify.
In order to make the Nginx container accessible
from the public internet,
[`nginx/main.js`](https://github.com/quilt/nginx/blob/master/main.js) explicitly
opens port 80 on the Nginx service to the outside world:

```javascript
webTier.allowFrom(publicInternet, 80);
```

This means you can
access the webpage you launched by copy-pasting the IP address from `quilt show`
into a browser window.  A site with "Hello, world!" text should appear.

Once you've launched a container, you'll often need to login to change something
or debug an issue.  The `quilt ssh` command makes this easy.  Use the container
ID in the `quilt show` output as the argument to `quilt ssh` to login to that
container. For instance, to ssh into a container or VM whose ID starts with
bd68:

```console
$ quilt ssh bd68
```

Note that you don't need to type the whole ID; as long as you use a unique
subset of it, Quilt will log in to the correct machine.

You may later decide that you'd like to change the contents of the simple
website.  You could do this by logging into the container, but for the sake of
example, let's do it using Quilt.  On your laptop, open the `nginx/index.html`
that was downloaded from github and change the contents of the page (e.g., you
can change the "Hello, World!" message).  Now, with the daemon still running,
re-deploy the webpage with Quilt:

```console
$ quilt run ./main.js
```

Quilt automatically detects the changes to your deployment, and will update the
cluster to implement your changes.  Note that we didn't need to tell Quilt to
stop the nginx container and start a new one; we just updated the view of what
the deployment should look like (in this case, by changing `index.html`), and
Quilt automatically detects this and updates the cluster accordingly.  Quilt
will prompt you to accept the changes that you're making to your deployment;
type `y`.  If you run `quilt show`, you'll notice that Quilt has stopped the old
container and is starting a new one.  If you navigate to the new IP address,
you'll notice your new page is up.

When you're done experimenting with Quilt, __make sure to stop the machines
you've started!__.  Otherwise, they will continue running on Amazon and you will
be charged for the unused time.  You can stop everything with Quilt's `stop`
command:

```console
$ quilt stop
```

You can use `quilt show` to ensure nothing is still running.  At this point, you
can kill the Quilt daemon.

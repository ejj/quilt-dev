# Blueprint Writers Guide

The previous section described how to use Quilt to run an application that
already had a blueprint. This guide describes how to write the Quilt
blueprint for a new application, using the lobste.rs application as an example.
lobste.rs is an open source project that implements a reddit-like web page,
where users can post content and vote up or down other content.

## Decomposing the application into containers

The first question you should ask yourself is "how should this application be
decomposed into different containers?"  Be sure you've read the [How Quilt
Works](#how-quilt-works) section, which gives a brief overview of containers.
If you've already figured out the containers that are needed for your
application (e.g., if you're already using Docker), you can skip the rest of
this section.

### Specifying the containers for your application

As an example of how to specify the containers for your application, let's use
the lobste.rs example.  lobste.rs requires mysql to run, so we'll use one
container for mysql.  We'll use a second container for the lobste.rs program to
run in.

For each container that your application uses, you'll need a container image.
The container image describes the filesystem that will be on the container when
it's started.  For mysql, for exampe, the container image includes all of the
dependencies that mysql needs to run, so that after starting a new mysql
container, you can simply launch mysql (no more installation is needed).  Most
popular applications already have containers that you can use, and a quick
google search yields an existing [mysql image](https://hub.docker.com/_/mysql/)
that we can use for lobste.rs.

For the container that runs lobste.rs, we'll need to create a new image by
writing our own Dockerfile, which describes how the Docker image should be
created.  In this case, the Dockerfile is relatively simple:

```docker
# This container is based on the Ruby image, which means that it
# automatically inherits the Ruby installation defined in that image.
FROM ruby:2.3.1

# Install NodeJS, which is required by lobste.rs.
RUN apt-get update && apt-get install nodejs -y

# Download and build the lobste.rs code.
RUN git clone git://github.com/jcs/lobsters.git
WORKDIR lobsters
RUN bundle

# Add a file to the container that contains startup code for lobste.rs. This
# command assumes that start-lobsters.sh is in the same directory as this
# Dockerfile.
COPY start-lobsters.sh /lobsters/

# When the container starts, it should run the lobste.rs server using the
# start-lobsters.sh bash file that we copied above.  This is a common
# "gotcha" to people new to containers: unlike VMs, each container is based
# on a process (in this case, rails, which is started at the end of
# start-lobsters.sh) and will be shutdown when that process stops.
ENTRYPOINT ["/bin/sh", "/lobsters/start-lobsters.sh"]
```

In this case, we wrote an additional bash script, [`start-lobsters.sh`](), to
help start the application.  The important thing about that script is that it
does some setup that needed to be done after the container was started, so it
couldn't be done in the Dockerfile.  For example, the first piece of setup it
does it to initialize the SQL database.  Because that requires a connection to
mysql, it needs to be done after the container is launched (and configured to
access the mysql container, as discussed below).  After initializing the
database, the `start-lobsters.sh` script launches the rails server, which is the
main process run by the container.

To create a docker image using this file, run `docker build` in the directory
with the Dockerfile (don't forget the period at the end!):

```console
$ docker build -t kayousterhout/lobsters .
```
    
In this case, we called the resulting image `kayousterhout/lobsters`, because
we'll push it to the Dockerhub for kayousterhout; you'll want to use your own
Dockerhub id to name your images.

This will take a few minutes, and creates a new image with the name
`kayousterhout/lobsters`.  If you want to play around with the new container,
you can use Docker to launch it locally:

```console
$ docker run -n lobsters-test kayousterhout/lobsters
```
    
To use a shell on your new container to poke around (while the `rails server` is
running), use:

```console
$ docker exec -it lobsters-test /bin/bash
```
    
This can be helpful for making sure everything was installed and is running as
expected (although in this case, lobste.rs won't work when you start it with
Docker, because it's not yet connected to a mysql container).

## Deploying the containers with Quilt

So far we have a mysql container image (we're using an existing one hosted on
Dockerhub) and a lobste.rs container image that we just made.  You should
similarly have the containers ready for your application.  Up until now, we
haven't done anything Quilt-specific: if you were using another container
management service like Kubernetes, you would have had to create the container
images like we did above.  These containers aren't yet configured to communicate
with each other, which is what we'll set up with Quilt.  We'll also use Quilt to
descrbie the machines to launch for the containers to run on.

To run the containers for your application with Quilt, you'll need to write a
Quilt blueprint.  Quilt blueprints are written in Javascript, and the Quilt
Javascript API
is described [here](https://github.com/quilt/quilt/tree/master/stitch).  In this
guide, we'll walk through how to write a Quilt blueprint for lobste.rs, but the
Quilt API has more functionality than we could describe here.  See the [API
guide](https://github.com/quilt/quilt/tree/master/stitch) for more usage
information.

### Writing the Quilt blueprint for MySQL

First, let's write the Quilt blueprint to get the MySQL container up and running.  We
need to create a container based on the mysql image:

```javascript
let sqlContainer = new Container('mysql:5.6.32');
```
    
Here, the argument to `Container` is the name of an image.  You can also pass in
a Dockerfile to use to create a new image, as described in the [Javascript API
documentation](https://github.com/quilt/quilt/tree/master/stitch).

Next, the SQL container requires some environment variables to be set.  In
particular, we need to specify a root password for SQL.  We can set the root
password to `foo` with the `setEnv` function:

```javascript
sqlContainer.setEnv('MYSQL_ROOT_PASSWORD', 'foo');
```
    
All containers need to be part of a service in order to be executed.  In this
case, the service just has our single mysql container.  Each service is created
using a name and a list of containers:

```javascript
let sqlService = new Service('sql', [sqlContainer]);
```
    
The SQL service is now initialized.  

### Writing the Quilt blueprint for lobste.rs

Next, we can similarly initialize the lobsters service.  The lobsters service is
a little trickier to initialize because it requires an environment variable
(`DATABASE_URL`) to be set to the URL of the SQL container.  Quilt containers
are each assigned unique hostnames when they're initialized, so we can create
the lobsters container and initialize the URL as follows:

```javascript
let lobstersContainer = new Container('kayousterhout/lobsters');
const sqlDatabaseUrl = 'mysql2://root:' + mysqlOpts.rootPassword + '@' + sqlService.hostname() + ':3306/lobsters';
lobstersContainer.setEnv('DATABASE_URL', sqlDatabaseUrl);
let lobstersService = new Service('lobsters', [lobstersContainer]);
```

### Allowing network connections
    
At this point, we've written code to create a mysql service and a lobsters
service.  With Quilt, by default, all network connections are blocked.  To allow
lobsters to talk to mysql, we need to explicitly open the mysql port (3306):

```javascript
lobstersService.connect(3306, sqlService);
```
    
Because lobsters is a web application, the relevant port should also be open to
the public internet on the lobsters service.  Quilt has a `publicInternet`
variable that can be used to connect services to any IP address:

```javascript
publicInternet.connect(3000, lobstersService);
```
    
### Deploying the application on infrastructure

Finally, we'll use Quilt to launch some machines, and then start our services on
those machines.  First, we'll define a "base machine."  We'll deploy a few
machines, and creating the base machine is a useful way to create one machine
that all of the machines in our deployment will be based off of.  In this case,
the base machine will be an Amazon instance that allows ssh access from the
public key "bar":

```javascript
let baseMachine = new Machine({provider: 'Amazon', sshKeys: ['ssh-rsa bar']});
```
    
Now, using that base machine, we can deploy a master and a worker machine.  All
quilt deployments must have one master, which keeps track of state for all of
the machines in the cluster, and 0 or more workers.  To deploy machines and
services, you must create a deployment object, which maintains state about the
deployment.

```javascript
const deployment = createDeployment();
deployment.deploy(baseMachine.asMaster());
deployment.deploy(baseMachine.asWorker());
```

We've now defined a deployment with a master and worker machine.  Let's finally
deploy the two services on that infrastructure:

```javascript
deployment.deploy(sqlService);
deployment.deploy(lobstersService);
```
    
We're done!  Running the blueprint is now trivial.  With a quilt daemon running, run
your new blueprint (which, in this case, is called lobsters.js):

```console
$ quilt run ./lobsters.js
```
    
Now users of lobsters, for example, can deploy it without needing to worry about
the details of how different services are connected with each other.  All they
need to do is to `quilt run` the existing blueprint.

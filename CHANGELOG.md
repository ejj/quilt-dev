Quilt Change Log
================

Up Next
-------------

- Package the OVS kernel module for the latest DigitalOcean image to speed up
boot times.
- Renamed specs to blueprints.
- Load balancing.
- Upgraded to the latest Docker engine version (17.05.0).
- Fixed bug in Google provider that caused ACLs to be repeatedly added.
- Fixed inbound connections on the Vagrant provider. In other words,
`myService.allowFrom(publicInternet, aPort)` now works.
- Only allocate one Google network per namespace, rather than one network for
each region within a namespace.
- Implement debugging counters accessible through `quilt counters`.

Release 0.1.0
-------------

Release 0.1.0 most notably modifies `quilt run` to evaluate Quilt specs using
Node.js, rather than within a Javascript implementation written in Go. This
enables users to make use of many great Node features, such as package management,
versioning, unit testing, a rich ecosystem of modules, and `use
strict`. In order to facilitate this, we now require `node` version 7.10.0 or
greater as a dependency to `quilt run`.

What's new:

- Fix a bug where Amazon spot requests would get cancelled when there are
multiple Quilt daemons running in the same Amazon account.
- Improve the error message for misconfigured Amazon credentials.
- Fix a bug where inbound and outbound public traffic would get randomly
dropped.
- Support floating IP assignment in DigitalOcean.
- Support arbitrary GCE projects.
- Upgrade to OVS2.7.
- Fix a race condition where the minion boots before OVS is ready.
- Build the OVS kernel module at runtime if a pre-built version is not
available.
- Evaluate specs using Node.js.

Release 0.0.1
-------------

Release 0.0.1 is an experimental release targeted at students in the CS61B
class at UC Berkeley.

Release 0.0.0
-------------

We are proud to announce the initial release of [Quilt](http://quilt.io)!  This
release provides an alpha quality implementation which can deploy a whole [host
of distributed applications](http://github.com/quilt) to Amazon EC2, Google
Cloud Engine, or DigitalOcean.  We're excited to begin this journey with our
inaugural release!  Please try it out and [let us
know](http://quilt.io/#contact) what you think.

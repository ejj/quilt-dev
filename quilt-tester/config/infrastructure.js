const { Machine } = require('@quilt/quilt');

/**
 * Handles deploying the necessary infrastructure to run the tests.
 * @param {number} nWorker Number of workers to use to run the tests
 */
function MachineDeployer(nWorker) {
  this.nWorker = nWorker;
}

MachineDeployer.prototype.deploy = function deploy(deployment) {
  const baseMachine = new Machine({
    provider: process.env.PROVIDER || 'Amazon',
    size: process.env.SIZE || 'm3.medium',
    sshKeys: ['ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCxMuzNUdKJREFgUkS' +
            'pD0OPjtgDtbDvHQLDxgqnTrZpSvTw5r8XDd+AFS6eVibBfYv1u+geNF3IEkpO' +
            'klDlII37DzhW7wzlRB0SmjUtODxL5hf9hKoScDpvXG3RBD6PBCyOHA5IJBTqP' +
            'GpIZUMmOlXDYZA1KLaKQs6GByg7QMp6z1/gLCgcQygTDdiTfESgVMwR1uSQ5M' +
            'RjBaL7vcVfrKExyCLxito77lpWFMARGG9W1wTWnmcPrzYR7cLzhzUClakazNJ' +
            'mfso/b4Y5m+pNH2dLZdJ/eieLtSEsBDSP8X0GYpmTyFabZycSXZFYP+wBkrUT' +
            'mgIh9LQ56U1lvA4UlxHJ'],
  });

  deployment.deploy(baseMachine.asMaster());

  const worker = baseMachine.asWorker();
  // Preemptible instances are currently only implemented on Amazon.
  if (baseMachine.provider === 'Amazon') {
    worker.preemptible = true;
  }

  deployment.deploy(worker.replicate(this.nWorker));
};

// We will have three worker machines.
module.exports = new MachineDeployer(3);

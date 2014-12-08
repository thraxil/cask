Cask is a simple content-addressed storage cluster with
a REST interface. Suitable as a building block for building
a more useful system on top of (eg, see [Reticulum](http://thraxil.github.io/reticulum/))

The simple public interface for any node in the cluster is:

    POST / --> post a file to the cluster. returns Key
    GET /file/<Key>/ -> retrieve a file based on the Key
    GET /status/ -> show node/cluster status

By default (for now), keys are SHA1 hashes of the files.

Additionally, nodes in the cluster communicate with each other over
HTTP.

    POST /local/ --> post a file to this node. returns Key
    GET /local/<Key>/ -> retrieve a file from this node by Key
    POST /join/ -> add a node to the cluster
    GET /info/ -> get JSON data about this node and its view of the
                  cluster

Features:

* Uploaded files are replicated across the cluster, placed to N nodes via a
  distributed hashtable.
* Nodes learn about cluster status via a Gossip protocol.
* An active anti-entropy process runs on each node, checking
  integrity and replication of stored files and balancing across the
  cluster.
* Pluggable Storage backends. Currently only disk is implemented, with
  plans for S3 next.

What Cask doesn't do:

* Cask stores no metadata whatsoever. Not even a mimetype. Data
  uploaded is just a binary blob that is returned as
  `application/octet`
* Cask cannot delete files. Once it's uploaded to a node, it's up.
* No security. Your cask server should be treated as an internal
  service and not be publically exposed.

These limitations are because Cask is meant to be a component in a
larger system.

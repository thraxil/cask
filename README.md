[![Build Status](https://travis-ci.org/thraxil/cask.svg?branch=master)](https://travis-ci.org/thraxil/cask)
[![Coverage Status](https://coveralls.io/repos/github/thraxil/cask/badge.svg?branch=master)](https://coveralls.io/github/thraxil/cask?branch=master)

Cask is a simple content-addressed storage cluster with
a REST interface. Suitable as a building block for building
a more useful system on top of (eg, see
[Reticulum](http://thraxil.github.io/reticulum/) or [hakmes](https://github.com/thraxil/hakmes))

The simple public interface for any node in the cluster is:

    POST / --> post a file to the cluster. returns Key
    GET / -> show basic info about the node/cluster
    GET /file/<Key>/ -> retrieve a file based on the Key
    GET /status/ -> show node/cluster status (JSON)

By default (for now), keys are SHA1 hashes of the files.

Additionally, nodes in the cluster communicate with each other over
HTTP.

    POST /local/ --> post a file to this node. returns Key
    GET /local/<Key>/ -> retrieve a file from this node by Key
    HEAD /local/<Key>/ -> find out if the node has this Key locally
    POST /join/ -> add a node to the cluster
    POST /heartbeat/ -> tell the node that I (another node) am alive
                        and well.

Features:

* Uploaded files are replicated across the cluster, placed to N nodes via a
  distributed hashtable.
* Nodes learn about cluster status via a Gossip protocol.
* An active anti-entropy process runs on each node, checking
  integrity and replication of stored files and balancing across the
  cluster.
* Read-repair. When you download a file from a node, it verifies the
  local copy and makes sure it is correctly balanced on the cluster.
* Pluggable Storage backends. Currently local disk, S3, and Dropbox
  are implemented, with plans for Google Drive, etc.

What Cask doesn't do:

* Cask stores no metadata whatsoever. Not even a mimetype. Data
  uploaded is just a binary blob that is returned as
  `application/octet`
* Cask cannot delete files. Once it's uploaded to a node, it's up.
* No security. Your cask server should be treated as an internal
  service and not be publically exposed.

These limitations are because Cask is meant to be a component in a
larger system.


Configuration
=============

12-factor style, each cask node is configured through environment
variables, all starting with `CASK_`. See the `env*` files in the test
directory for examples of a simple cluster's settings. The ones that it expects:

CASK_PORT
---------

Port to listen on.

CASK_BASE_URL
-------------

Public base url. Leave off the trailing slash. eg,
`http://localhost:8080`

CASK_UUID
---------

Unique ID for the node. Every node in the cluster MUST have a unique
ID. This doesn't strictly have to be a UUID, but it's recommended. An
easy way to generate a unique id for each node is with

    python -c "import uuid; print uuid.uuid4()"

Try not to change these during the life of the cluster. The UUID is
also the key used to determine which segments of the ring that node
claims. So if you change the UUID after files have been written to it,
many of them will likely have to move.

CASK_WRITEABLE
--------------

Is this node writeable? If not, it will be considered read-only. This
is useful if a node has filled up a disk. You can set it to read-only
and still serve files from it, but it won't accept any new ones.

CASK_BACKEND
------------

What is the storage backend for the node. Currently only 'disk' is implemented.

CASK_DISK_BACKEND_ROOT
----------------------

Root directory for the disk storage backend. Full path is
recommended. Obviously the user that the node is running as must have
read and write permissions to it.

CASK_NEIGHBORS
--------------

A comma seperated list of base URLs for other nodes. If this exists,
the cask node will try, upon startup, to join those other nodes. This
is handy for bootstrapping the cluster.

CASK_REPLICATION
----------------

How many nodes to attempt to replicate to. You will want to have at
least this many (writeable) nodes in your cluster. If it can't write a
file to this many nodes, it will fail on upload and complain.

CASK_MAX_REPLICATION
--------------------

As nodes come and go, sometimes you get extra copies of files on nodes
that aren't at the front of the list for a given key. The active
anti-entropy system will clear them out from the excess nodes if there
are more than this many copies. Must be higher than
`CASK_REPLICATON`, but you probably don't want it *much* higher.

CASK_CLUSTER_SECRET
-------------------

Shared secret key for the cluster. Every node must be configured with
exactly the same value for this field.

CASK_HEARTBEAT_INTERVAL
-----------------------

How many seconds to sleep in between heartbeats. On each heartbeat, a
node wakes up and sends a heartbeat signal to all the neighbors that
it knows about to let them know it's still alive. Set this low enough
that a dead node will be detected fairly quickly, but not so low that
you waste a ton of bandwidth with heartbeats.

CASK_AAE_INTERVAL
-----------------

How many seconds to sleep in between active anti-entropy file
checks. This interval times the number of files stored on each node
will be roughly how long it takes to verify and rebalance your entire
repository. So think about how important that refresh period is and
balance it against how much CPU and bandwidth the AAE system will
consume.

CASK_MAX_PROCS
--------------

Maximum number of CPUs that can be executing simultaneously. Defaults
to the number of CPU cores on your system. Set it lower if you want to
reduce concurrency for some reason.

CASK_READ_TIMEOUT
-----------------

Max read timeout for HTTP(S) server. Defaults to 5 (seconds).

CASK_WRITE_TIMEOUT
------------------

Max write timeout for HTTP(S) server. Defaults to 20 (seconds). If you
serve really large files out of your cluster, you may need to increase
this.

CASK_SSL_CERTIFICATE and CASK_SSL_KEY
-------------------------------------

Paths to certificate and key files. If you set these, you must also
set your BASE_URL to start with 'https://'. This will cause Cask to
serve via TLS. Otherwise, you get plain HTTP.

Be careful of self-signed certificates and such. Go's TLS client
library is very picky about that sort of thing.

CASK_S3_ACCESS_KEY, CASK_S3_SECRET_KEY, and CASK_S3_BUCKET
----------------------------------------------------------

To use S3 storage, you must set the `CASK_BACKEND` to 's3' and put in
appropriate values for these.


CASK_DROPBOX_ACCESS_KEY, CASK_DROPBOX_SECRET_KEY, and CASK_DROPBOX_TOKEN
------------------------------------------------------------------------

To use dropbox storage, you must set the `CASK_BACKEND` to "dropbox"
and put in appropriate values for these. To configure, you will
need to log in to dropbox and register an app at:
https://www.dropbox.com/developers/apps/create

That will give you at least the access key and secret key. If you
start the cask node without the token set, it will send you to a
page to complete the authorization and give you the token.


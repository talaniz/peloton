The Peloton Unified Resource Scheduler
=======================================

Peloton is a Unified Resource Scheduler built on top of Mesos to
manage compute resources more efficiently for diverse workloads such
as batch, stateless, stateful and daemon jobs. Peloton provides
hierarchical max-min fairness guarantees for different
organizations. It has a few unique design priciples that
differentiates itself from other Mesos schedulers:

1. Scalable and high-avaiable architecture. Unlike other
active-standby schedulers such as Aurora and Marathon, Peloton uses a
scale-out and all-active achitecture where all Peleoton job managers
are able to handle read and write requests concurrently.

2. Support different types of workloads such as batch, stateless,
stateful and daemon jobs. All those workloads can be colocated on a
shared cluster so that we can improve the overall cluster utilization
while provide resource isolation for different workloads.

3. Support hierachical resource pools for elastic resource sharing
among organizations in a shared cluster. The resource pools in Peloton
provide hierarchical max-min fairness guarantees for organizations.

4. Support GPU scheduling for a cluster with mixed GPU and non-GPU
hosts. Any GPU jobs such as TensorFlow or Caffe will be scheduled and
launched to hosts with corresponding GPU resources.

5. Support UNS based service discovery and routing via
Mesos-UNS-Bridge so that the Peloton tasks can be discovered via UNS
or routed to via routing agents like HaProxy or Muttley.

6. Support persistent or immutable volumes for a job. For example,
uConfig or translations data can be mounted into a job instance using
a remote volume referenced by a immutable URL to a
udeploy-replicator. Peloton will use the same job upgrade workflow to
upgrade an uConfig version by simplying changing the URL of a remote
volume.

[Project Page] (http://t.uber.com/peloton)
[Runbook](https://code.uberinternal.com/w/runbooks/peloton/)

## Install

Installations of protoc/proto/protoc-gen-go are required, run
bootstrap.sh once so all build dependencies will be installed.  Want
to build debian package or docker image ? Follow packaging/README.md

cd $GOPATH

mkdir -p src/code.uber.internal/infra/

git clone gitolite@code.uber.internal:infra/peloton src/code.uber.internal/infra/peloton

cd $GOPATH/src/code.uber.internal/infra/peloton

( run bootstrap.sh only once )

./bootstrap.sh

glide install

make

## Run pcluster to bootstrap runtime dependencies like mesos and db
make pcluster


## Run Peloton master/apps in containers
Please refer to tools/pcluster/README.md for more details

## Test Peloton apps
Create new job via yarpc based go client:

cd $GOPATH/src/code.uber.internal/infra/peloton

bin/peloton job create test test/testjob.yaml --master http://localhost:5292

bin/peloton task list test --master http://localhost:5292


## Run Peloton master

To run peloton in dev environment, dependencies like mesos/mysql, need
to be set up first.  Run 'make pcluster' to bootstrap those
dependencies in containers (docker-py installation is required, see
bootstrap.sh for more details).  Refer to "docker/README.md" for
details.

./bin/peloton-master -c config/master/base.yaml -c config/master/development.yaml -d

By default, it runs peloton master at port 5289. To run another
peloton master instance, set env var 'PORT=5290', or pass the `--port`
flag.


## Run peloton from docker container

### Build

```
$ make docker
...
Built uber/peloton:51f1c4f
```

If you want to build an image with a different name: `IMAGE=foo/bar:baz make docker`

### Run

The docker container takes a few environment variables to configure how it will run. Each peloton
app is launchable by setting `APP=$name` in the environment. For example, to run the
peloton-master, execute:

```
$ docker run --rm --name peloton -it -p 5289:5289 -e APP=master -e ENVIRONMENT=development peloton
```

Configurations are stored in `/etc/peloton/$APP/`, and by default we will pass the following
arguments: `-c "/etc/peloton/${APP}/base.yaml" -c "/etc/peloton/${APP}/${ENVIRONMENT}.yaml"`

NOTE: you need to make sure the container has access to all the dependencies it needs, like mesos-master,
zookeeper, mysql, cassandra, etc. Check your configs!

#### Master with pcluster

```
$ make pcluster
$ docker run --rm --name peloton -it -e APP=master -e ENVIRONMENT=development --link peloton-mesos-master:mesos-master --link peloton-zk:zookeeper --link peloton-mysql:mysql --link peloton-cassandra:cassandra peloton
```

#### Client

Launching the client is similar (replace `-m` argument with whereever your peloton-master runs:

```
$ docker run --rm -it --link peloton:peloton -e APP=client peloton job -m http://peloton:5289/ create test1 test/testjob.yaml
```



## Packaging

Build debs for supported distributions. Output will be placed into `./debs`. You can specify
the DISTRIBUTION by passing `DISTRIBUTION=jessie` (jessie and trusty are supported). Defaults
to `all`.

```
$ make debs
```

## Pushing docker containers

`make docker-push` will build docker containers, and push them to both ATG and
SJC1 registries. You can push to only one DC with `DC=atg` or `DC=sjc1`. You can
override the image to push with `IMAGE=foo/bar:baz`

To build and deploy docker containers everywhere:

```
make docker docker-push
```


## Test Peloton master

1. Create new job via yarpc based go client:

cd $GOPATH/src/code.uber.internal/infra/peloton

bin/peloton job create test test/testjob.yaml

bin/peloton task list test


2. Curl into Peloton endpoint:

curl -X POST  \
     -H 'content-type: application/json'  \
     -H 'Rpc-Procedure: JobManager.Get'   \
     -H 'Rpc-Service: peloton-master'     \
     -H 'Rpc-Caller: peloton-client'      \
     -H 'Context-TTL-MS: 1000'            \
     -H 'Rpc-Encoding: json'              \
     --data '{"id": {"value": "myjob12345"}}' 	\
    localhost:5289/api/v1

## Debug Peloton Apps in Docker Container


1. Find docker container process ID:
sudo docker inspect -f {{.State.Pid}} <DOCKER_IMAGE_ID>

2. Run a bash shell in the container:
nsenter -t <PID> -m -p bash

3. Setup source code directory symlink:
mkdir -p /workspace/src/code.uber.internal/infra/
ln -s /peloton-install /workspace/src/code.uber.internal/infra/peloton

4. Start the gdb in the bash shell:
gdb peloton-install/bin/peloton-[hostmgr|jobmgr|resmgr|placement] <PID>

5. Happy debugging ;-)

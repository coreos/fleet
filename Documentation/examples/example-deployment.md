# Deploying a Service Using fleet

The following is an example of how one might deploy a load-balanced web service using fleet. 
This example deploys [subgun](https://github.com/coreos/subgun), a simple subscription tool for [mailgun](https://mailgun.com/). 

subgun is deployed in two pieces: an application and a presence daemon. The application simply serves HTTP requests through an AWS load balancer, while the presence daemon updates the load balancer with backend information. The diagram below illustrates this model:

![image](img/subgun.png)

## Containers

Both components are deployed in [docker](https://www.docker.io/) containers running on CoreOS. The following Dockerfiles were each built and their resulting images pushed to the public docker index:

**coreos/subgun**

```
FROM stackbrew/ubuntu:precise
RUN apt-get install -y --allow-unauthenticated ca-certificates

ADD bin/subgun /bin/

EXPOSE 8080
ENV SUBGUN_LISTEN 127.0.0.1:8080
CMD /bin/subgun
```

**quay.io/coreos/elb-presence**

```
FROM ubuntu:14.04

RUN apt-get update
RUN apt-get install -y python-boto

ADD elb-presence /bin/elb-presence

CMD /bin/elb-presence
```

## Service Files

With the docker images available over the public internet, systemd can simply run the containers. 

The following unit files are [templates](https://github.com/coreos/fleet/blob/master/Documentation/unit-files-and-scheduling.md#template-unit-files), which means they can be run multiple times by referencing them with full instance names. You can find these unit files in the [unit-examples](https://github.com/coreos/unit-examples/tree/master/blog-fleet-intro) repository. To save time, clone the repo on the machine from which you are controlling fleet.

**`subgun-http@.service`**

```ini
[Unit]
Description=subgun

[Service]
ExecStartPre=-/usr/bin/docker kill subgun-%i
ExecStartPre=-/usr/bin/docker rm subgun-%i
ExecStart=/usr/bin/docker run --rm --name subgun-%i -e SUBGUN_LISTEN=127.0.0.1:8080 -e SUBGUN_LISTS=recv@sandbox2398.mailgun.org -e SUBGUN_API_KEY=key-779ru4cibbnhfa1qp7a3apyvwkls7ny7 -p 8080:8080 coreos/subgun
ExecStop=/usr/bin/docker stop subgun-%i

[X-Fleet]
Conflicts=subgun-http@*.service
```

**`subgun-presence@.service`**

```ini
[Unit]
Description=subgun presence service
BindsTo=subgun-http@%i.service

[Service]
ExecStartPre=-/usr/bin/docker kill subgun-presence-%i
ExecStartPre=-/usr/bin/docker rm subgun-presence-%i
ExecStart=/usr/bin/docker run --rm --name subgun-presence-%i -e AWS_ACCESS_KEY=AKIAIBC5MW3ONCW6J2XQ -e AWS_SECRET_KEY=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 -e AWS_REGION=us-east-1 -e ELB_NAME=bcwaldon-fleet-lb quay.io/coreos/elb-presence
ExecStop=/usr/bin/docker stop subgun-presence-%i

[X-Fleet]
MachineOf=subgun-http@%i.service
```

If you are going to modify these units, be sure you don't copy a `docker run` command that starts a container in detached mode (`-d`). Detached mode won't start the container as a child of the unit's pid. This will cause the unit to run for just a few seconds and then exit.

## Deploy!


First, load the unit file templates into fleet:

```sh
$ fleetctl submit subgun-presence@.service subgun-http@.service
$ fleetctl list-unit-files
UNIT				HASH	DSTATE		STATE		TMACHINE
subgun-http@.service		08be408	inactive	inactive	-
subgun-presence@.service	5180ed9	inactive	inactive	-
```

And now, using shell expansion, we can easily launch three instances of each template:

```sh
$ fleetctl start subgun-presence@{1..3}.service subgun-http@{1..3}.service
Unit subgun-http@1.service launched on 0e0a1f59.../172.17.8.102
Unit subgun-http@2.service launched on 30a73182.../172.17.8.103
Unit subgun-presence@1.service launched on 0e0a1f59.../172.17.8.102
Unit subgun-presence@2.service launched on 30a73182.../172.17.8.103
Unit subgun-http@3.service launched on 610a163d.../172.17.8.101
Unit subgun-presence@3.service launched on 610a163d.../172.17.8.101
$ fleetctl list-units
UNIT				MACHINE				ACTIVE	SUB
subgun-http@1.service		0e0a1f59.../172.17.8.102	active	running
subgun-http@2.service		30a73182.../172.17.8.103	active	running
subgun-http@3.service		610a163d.../172.17.8.101	active	running
subgun-presence@1.service	0e0a1f59.../172.17.8.102	active	running
subgun-presence@2.service	30a73182.../172.17.8.103	active	running
subgun-presence@3.service	610a163d.../172.17.8.101	active	running
```

At this point, our application is deployed!

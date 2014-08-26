# Deploying a Service Using fleet

The following is an example of how one might deploy a load-balanced web service using fleet. 
This example deploys [subgun](https://github.com/coreos/subgun), a simple subscription tool for [mailgun](https://mailgun.com/). 

subgun is deployed in two pieces: an application and a presence daemon. The application simply serves HTTP requests through an AWS load balancer, while the presence daemon updates the load balancer with backend information. The diagram below illustrates this model:

![image](subgun.png)

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

**coreos/elb-presence**

```
FROM stackbrew/ubuntu:precise

RUN apt-get update
RUN apt-get install -y python-requests python-boto

ADD bin/elb-presence /bin/elb-presence

CMD /bin/elb-presence
```

## Service Files

With the docker images available over the public internet, systemd can simply run the containers. The following templates are rendered with all configuration information and can be run multiple times by incrementing the integer in the filename and within the unit. You can find these unit files in the [unit-examples](https://github.com/coreos/unit-examples/tree/master/blog-fleet-intro) repository. To save time, clone the repo on the machine from which you are controlling fleet.

**subgun-http.1.service**

```
[Unit]
Description=subgun

[Service]
ExecStartPre=-/usr/bin/docker kill subgun-1
ExecStartPre=-/usr/bin/docker rm subgun-1
ExecStart=/usr/bin/docker run --rm --name subgun-1 -e SUBGUN_LISTEN=127.0.0.1:8080 -e SUBGUN_LISTS=recv@sandbox2398.mailgun.org -e SUBGUN_API_KEY=key-779ru4cibbnhfa1qp7a3apyvwkls7ny7 -p 8080:8080 coreos/subgun
ExecStop=/usr/bin/docker stop subgun-1

[X-Fleet]
X-Conflicts=subgun-http.*.service
```

**subgun-presence.1.service**

```
[Unit]
Description=subgun presence service
BindsTo=subgun-http.1.service

[Service]
ExecStartPre=-/usr/bin/docker kill subgun-presence-1
ExecStartPre=-/usr/bin/docker rm subgun-presence-1
ExecStart=/usr/bin/docker run --rm --name subgun-presence-1 -e AWS_ACCESS_KEY=AKIAIBC5MW3ONCW6J2XQ -e AWS_SECRET_KEY=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 -e AWS_REGION=us-east-1 -e ELB_NAME=bcwaldon-fleet-lb coreos/elb-presence
ExecStop=/usr/bin/docker stop subgun-presence-1

[X-Fleet]
X-ConditionMachineOf=subgun-http.1.service
```

## Deploy!


At this point, it is simple enough to hand the unit files over to fleet:

```
$ fleetctl submit subgun-presence.*.service subgun-http.*.service
$ fleetctl list-unit-files
UNIT				HASH	DSTATE		STATE		TMACHINE
subgun-http.1.service		9645ede	inactive	inactive	-
subgun-http.2.service		dc866b0	inactive	inactive	-
subgun-http.3.service		43644e7	inactive	inactive	-
subgun-presence.1.service	3a52ed1	inactive	inactive	-
subgun-presence.2.service	37811b3	inactive	inactive	-
subgun-presence.3.service	b1ba9e2	inactive	inactive	-
```

And now they can be started:

```
$ fleetctl start subgun-presence.*.service subgun-http.*.service
$ fleetctl list-units
UNIT				MACHINE				ACTIVE	SUB
subgun-http.1.service		16b3e287.../172.17.8.101	active	running
subgun-http.2.service		4127c5ef.../172.17.8.103	active	running
subgun-http.3.service		ec4ba903.../172.17.8.102	active	running
subgun-presence.1.service	16b3e287.../172.17.8.101	active	running
subgun-presence.2.service	4127c5ef.../172.17.8.103	active	running
subgun-presence.3.service	ec4ba903.../172.17.8.102	active	running
```

At this point, our application is deployed!

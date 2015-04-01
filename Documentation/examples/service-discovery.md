# Service Discovery

Since applications and networking environments vary widely between customer deployments, flt does not provide a generalized, integrated solution for service discovery. However, there are a number of patterns available which can easily be implemented on top of flt to provide automated and reliable service discovery. One such pattern, the _sidekick model_, is described below.

## Sidekick model

The sidekick model works in a very similar fashion to [Synapse](https://github.com/airbnb/synapse), which runs a separate discovery agent next to the main container that is being run. This can be easily accomplished in flt with the `MachineOf` option.

Instead of guessing when an application is healthy and ready to serve traffic, you can write agent to be as simple or complex as you see fit. For example, your agent might want to check the applications `/v1/health` endpoint after deployment before declaring the instance healthy and announcing it. For another application, you might want to announce each instance by public IP address intead of a private IP.

## Sample Discovery Unit

Here's an extremely simple bash agent that blindly announces our Nginx container after it is started:

```
[Unit]
Description=Announce nginx1.service
# Binds this unit and nginx1 together. When nginx1 is stopped, this unit will be stopped too.
BindsTo=nginx1.service

[Service]
ExecStart=/bin/sh -c "while true; do etcdctl set /services/website/nginx1 '{ \"host\": \"%H\", \"port\": 8080, \"version\": \"52c7248a14\" }' --ttl 60;sleep 45;done"
ExecStop=/usr/bin/etcdctl delete /services/website/nginx1

[X-Flt]
# This unit will always be colocated with nginx1.service
MachineOf=nginx1.service
```

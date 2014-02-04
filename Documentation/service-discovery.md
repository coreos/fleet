# Service Discovery

Service discovery with coreinit will be handled in a very similar fashion to [Synapse](https://github.com/airbnb/synapse), which runs a separate discovery agent next to the main container that is being run. While this functionality will be implemented in a higher order system above coreinit, you can configure your own units to have discovery agents run beside them with the `X-ConditionMachineOf` option.

It's hard for coreinit to generalize the logic for service discovery when applications and networking environments vary widely between each customer's deployment. Instead of guessing when an application is healthy and ready to serve traffic, you can write agent to be as simple or complex as you see fit.

For example, your agent might want to check the applications `/v1/health` endpoint after deployment before declaring the instance healthy and announcing it. For another application, you might want to announce each instance by public IP address intead of a private IP.

## Sample Discovery Unit

Here's an extremely simple bash agent that blindly announces our Nginx container after it is started:

```
[Unit]
Description=Announce nginx1.service
# Binds this unit and nginx1 together. When nginx1 is stopped, this unit will be stopped too.
BindsTo=nginx1.service

[Service]
ExecStart=/bin/sh -c "while true; do etcdctl set /services/website/nginx1 '{ \"host\": \"10.10.10.2\", \"port\": 8080, \"version\": \"52c7248a14\" }' --ttl 60;sleep 45;done"
ExecStop=/usr/bin/etcdctl delete /services/website/nginx1

[X-Coreinit]
# This unit will always be colocated with nginx1.service
X-ConditionMachineOf=nginx1.service
```

FROM alpine:latest

ADD fleetd /usr/local/bin/
ADD fleetctl /usr/local/bin/

ARG FLEET_IMG_VER
ENV FLEET_IMG_VER ${FLEET_IMG_VER:-v0.13}
RUN echo $FLEET_IMG_VER

# Define default command.
CMD ["/usr/local/bin/fleetd"]

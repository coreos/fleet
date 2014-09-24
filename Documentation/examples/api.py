#
# This file provides an example of a very simple client library written in Python.
# The client builds an interface for interacting with the fleet API, then retrieves
# a list of Units currently loaded into fleet.
#
# Warning: the code below is a significally simplified version of a typical client
# library. It is an incomplete implementation that is provided to demonstrate
# some aspects of building a client library. It is not production-ready code.
#
# This example assumes that fleet is configured to listen on localhost:8080
#
# Requirements:
#  httplib2 - https://github.com/jcgregorio/httplib2
#  uritemplate - https://github.com/uri-templates/uritemplate-py
#
import httplib2
import json
import uritemplate
import urllib
import urlparse
import pprint

# Step 1: Fetch Discovery document.
ROOT_URL = "http://localhost:8080/"
DISCOVERY_URI = ROOT_URL + "v1-alpha/discovery.json"
h = httplib2.Http()
resp, content = h.request(DISCOVERY_URI)
discovery = json.loads(content)

# Step 2.a: Construct base URI
BASE_URI = ROOT_URL + discovery['servicePath']

class Collection(object): pass

def createNewMethod(name, method):
  # Step 2.b Compose request
  def newMethod(**kwargs):
    body = kwargs.pop('body', None)
    url = urlparse.urljoin(BASE_URI, uritemplate.expand(method['path'], kwargs))
    for pname, pconfig in method.get('parameters', {}).iteritems():
      if pconfig['location'] == 'path' and pname in kwargs:
        del kwargs[pname]
    if kwargs:
      url = url + '?' + urllib.urlencode(kwargs)
    return h.request(url, method=method['httpMethod'], body=body,
                     headers={'content-type': 'application/json'})

  return newMethod

# Step 3.a: Build client surface
def build(discovery, collection):
  for name, resource in discovery.get('resources', {}).iteritems():
    setattr(collection, name, build(resource, Collection()))
  for name, method in discovery.get('methods', {}).iteritems():
    setattr(collection, name, createNewMethod(name, method))
  return collection

service = build(discovery, Collection())

# Step 3.b: Use the client
response = service.Machines.List()

# output metadata (status, content-length, etc...)
pprint.pprint(response[0])

# output body
pprint.pprint(json.loads(response[1]))


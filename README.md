Honeycomb AppEngine http.HandlerFunc Wrapper
======

The default `hnynethttp.WrapHandlerFunc` doesn't work in AppEngine Standard.

The reason is because in AppEngine Standard you have to use the URLFetch service
to make any outgoing requests. When the default libhoney package tries to send
events to the Honeycomb API, nothing happens because it's using the default HTTP
client from the `http` library.

This is a crude implementation of an AppEngine Standard `WrapHandlerFunc`
wrapper. It's crude because it doesn't do any nice batching or anything like
that -- every time a request comes in it's immediately sent.

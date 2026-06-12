# Retrospective — 30 Days of Go

I came into this with production backend experience but scattered, half-forgotten Go and
no real project to show for it. Over 30 days I built a complete notification dispatcher
service with REST API, concurrent worker pool, retry logic, graceful shutdown, tests, Docker.

The biggest shift was that Go has no magic: every dependency, every error, every goroutine 
is explicit. It is more verbose than Java, but I always know what is happening and why and 
that transparency is what made the fundamentals real for me.

I am still growing, but I now have the confidence that I can go deep when given something real to build.
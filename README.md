# simple-http-proxy
A very basic self-hosted HTTP proxy.

## Usage
The executable accepts two parameters:
```
-listen string
	Listen address for server, eg. 127.0.0.1:8080 (default "localhost:8080")
-users string
	Users configuration file (default "users.json")
```

Only the `-users` flag is required for minimal use. Users is a JSON file that contains
the user list and their assigned quotas. See `users.example.json`.

## Issues
There is an issue that I don't want to spend time trying to fix anymore.
It's related to a non-working test. See `proxy_test.go` for details.

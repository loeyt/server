# loeyt-server

Work In Progress.

## Environment variables

| Name | Description | Default |
| --- | --- | --- |
| ACME_CACHE | Directory to place ACME cache files | Current work directory |
| ACME_WHITELIST | A colon-separated list of domains | Switches to Let's Encrypt staging |
| HTTP_PORT | Port number to use when not socket-activated | 8080 |
| LISTEN_FDS | Systemd socket activation | Listen on port 8080 |
| LISTEN_FDNAMES | Systemd socket activation | See notes |

## Systemd socket activation

Using systemd you can bind to http and https ports. To know which port is
which, it expects the ports to be named `http` and `https`. They don't have to
be port 80 and port 443, but the server assumes requests coming in through the
http socket can be redirected to the same host with no changes except for URL
scheme.

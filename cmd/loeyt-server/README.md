# loeyt-server

Work In Progress.

## Environment variables

| Name | Description | Default |
| --- | --- | --- |
| ACME_CACHE | Directory to place ACME cache files | Current work directory |
| ACME_WHITELIST | A colon-separated list of domains | Switches to Let's Encrypt staging |
| HTTP_PORT | Port number to use when not socket-activated | 8080 |
| LISTEN_FDS | Systemd socket activation | Listen on $HTTP_PORT |

## Systemd socket activation

Using systemd you can bind to http and https ports. To do this, use socket activation. The first socket is assumed to be 
https. An optional second socket is assumed to be http and is used to redirect to https.

# Queuer
Service to manage queue from system.

# Getting Started

First step is to create a `.env` file in project root and change example values to your config. You can use `example.env` file from `_setup` folder as template.

And then, run `docker compose up -d` into project root to start project.

## Tests

The easy way to run tests is just run `docker compose up -d` command to start project with variables. So, enter in `synk_queuer` with `docker exec` and run `go test ./tests -v`.

## Certificates

This app must run in HTTPS to authentication works properly. So, to install it, just setup `[mkcert](https://github.com/FiloSottile/mkcert)` into your machine and then run command below into root directory of this project.

```
mkcert -key-file ./.cert/key.pem -cert-file ./.cert/cert.pem localhost synk_queuer
```

## Network

You can use a custom network for this services, using then `synk_network` you must create before run it. So, to create on just run command below once during initial setup.

```
docker network create synk_network
```

# Routes

## Get info about app

> `GET` /about

### Response

```json
{
	"ok": true,
	"error": "",
	"info": {
		"server_port": "8080",
		"app_port": "8083",
		"db_working": true
	},
	"list": null
}
```
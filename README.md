# Devoid
## Development
`devoid` is split into two segments, the server and terminal client. Both reside
within the same repo. 

In order to contribute, you must get the code, setup a self-signed certificate
(for running the server with TLS), and generate some basic data.

**Get the Repo**

```bash
go get github.com/clagraff/devoid
```

**Create `devoid` directory**
```bash
mkdir -p ~/.config/devoid
```

### Server Setup

**Create cert for server**

```bash
openssl genrsa -out ~/.config/devoid/devoid.key 2048
openssl req -new -x509 -sha256 -key ~/.config/devoid/devoid.key -out ~/.config/devoid/devoid.crt -days 3650
```

**Create Server Config**

```bash
echo '{"certPath":"/home/USER/.config/devoid/devoid.crt","entitiesPath":"/home/USER/.config/devoid/entities.json","keyPath":"/home/USER/.config/devoid/devoid.key"}' > ~/.config/devoid/server.json
```

**Create Entities**
```bash
echo '[{"ID":"7e874935-c241-4a40-8c71-54ac6d6c3eff","Position":{"X":3,"Y":7},"Spatial":{"OccupiesPosition":true,"Stackable":false}},{"ID":"8e50e77b-dca9-4cb8-b228-c127b04442e7","Position":{"X":5,"Y":1},"Spatial":{"OccupiesPosition":true,"Stackable":false}}]' > ~/.config/devoid/entities.go
```

**Run the server**
```bash
go run cmd/server/main.go ~/.config/devoid/server.json
```

### Client Setup
**Create Client Config**
```bash
echo '{"clientID":"7e874935-c241-4a40-8c71-54ac6d6c3eff","entityId":"7e874935-c241-4a40-8c71-54ac6d6c3eff"}' > ~/.config/devoid/client.json
```

**Run the client**
```bash
go run cmd/client/main.go ~/.config/devoid/client.json
```

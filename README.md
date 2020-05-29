# nitr agent
Nitr is a webserver that collects System and Hardware information and makes it accessible through an JSON API

### Building from source
#### Clone
```
git clone git@github.com:juanhuttemann/nitr-agent.git
```
#### Build
```
cd nitr-agent
go build
```

### Using Nitr Agent

**Linux**
```
./nitr-agent
```

**Windows**
You can double click the .exe file or type in cmd
```
nitr-agent.exe
```
the server will start listening on port 8000 by default

### Accessing web panel
Go to your web browser en open [http://localhost:8000](http://localhost:8000)

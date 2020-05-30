<p align="center">
    <img alt="Nitr" height="125" src="https://raw.githubusercontent.com/juanhuttemann/nitr-agent/master/app/assets/images/logo.png" style="max-width:100%;">
    <br>
</p>

[![Build Status](https://travis-ci.org/juanhuttemann/nitr-agent.svg?branch=master)](https://travis-ci.org/juanhuttemann/nitr-agent)
[![MIT license](https://img.shields.io/badge/License-MIT-blue.svg)](https://github.com/juanhuttemann/nitr-agent/blob/master/LICENSE)

# nitr agent
Nitr is a webserver that collects System and Hardware information and makes it accessible through an JSON API

### Building from source
Note: go version 1.13 or higher is required building it from the source.

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
Go to [http://localhost:8000](http://localhost:8000) in your web browser

![preview](https://raw.githubusercontent.com/juanhuttemann/nitr-agent/master/images/login-web.png)

Access with default **username** and **password**: **admin admin**

![preview](https://raw.githubusercontent.com/juanhuttemann/nitr-agent/master/images/panel-web.png)

# API

## Available Endpoints

These endpoints allow you to get system and hardware information about your host.

| Verb   | Endpoint                      | Data                         |
|--------|-------------------------------|------------------------------|
|GET     | /cpu                          | CPU                          |
|GET     | /bios                         | Bios                         |
|GET     | /bandwidth                    | Bandwidth                    |
|GET     | /chassis                      | Chassis                      |
|GET     | /disks                        | Disks                        |
|GET     | /drives                       | Drives                       |
|GET     | /devices                      | Devices (Linux Only)         |
|GET     | /gpu                          | GPU                          |
|GET     | /network                      | Network                      |
|GET     | /processes                    | Processes                    |
|GET     | /ram                          | RAM                          |
|GET     | /baseboard                    | Baseboard                    |
|GET     | /product                      | Product                      |


## How to Use

Call the above endpoints with ?key=secret in the URL or pass the x-api-key header with value secret you will get success response.

### Examples:

Requesting CPU Information

```
curl -X Get 'http://localhost:8000/api/v1/cpu' -H 'x-api-key:secret'
```
Response

```json
{
	"brand":"Intel(R) Core(TM) i7-4810MQ CPU @ 2.80GHz",
	"cores":4,
	"threads":8,
	"frecuency":3800,
	"usage":8.354430379674321,
	"usageEach":[
				9.803921568623954,
				7.692307692348055,
				4.166666666635087,
				4.166666666698246,
				6.122448979565321,
				6.12244897961267,
				4.081632653074482,
				5.88235294118696
	]
}
```


## Powered by

* [Fiber](https://gofiber.io/) - The web framework used
* [bbolt](https://github.com/etcd-io/bbolt) - Database
* [UIKit](https://getuikit.com/) - Front-End framework

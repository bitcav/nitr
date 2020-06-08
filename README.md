﻿
<p align="center">
    <img alt="Nitr" height="125" src="https://raw.githubusercontent.com/juanhuttemann/nitr-agent/master/app/assets/images/logo.png" style="max-width:100%;">
    <br>
</p>

![Release](https://raw.githubusercontent.com/juanhuttemann/nitr-agent/master/images/release.svg)
[![Build Status](https://travis-ci.org/juanhuttemann/nitr-agent.svg?branch=master)](https://travis-ci.org/juanhuttemann/nitr-agent)
[![Go Report Card](https://goreportcard.com/badge/github.com/juanhuttemann/nitr-agent)](https://goreportcard.com/report/github.com/juanhuttemann/nitr-agent)
[![MIT license](https://img.shields.io/badge/License-MIT-blue.svg)](https://github.com/juanhuttemann/nitr-agent/blob/master/LICENSE)

# nitr-agent
nitr-agent is a cross-platform remote monitoring tool written in Golang, providing system and hardware information through a JSON API.

<p>
    <img alt="Nitr" src="https://raw.githubusercontent.com/juanhuttemann/nitr-agent/master/images/usage.gif" style="max-width:100%;">
</p>

> curl + jq demo

## Installation

### Download

https://github.com/juanhuttemann/nitr-agent/releases/latest

### Building from source
Note: go version 1.13 or higher is required building it from the source.

#### Clone
```
git clone https://github.com/juanhuttemann/nitr-agent.git
```
#### Build
```
cd nitr-agent
go build
```

### Running

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

<p>
    <img alt="app" src="https://raw.githubusercontent.com/juanhuttemann/nitr-agent/master/images/app-start.gif" style="max-width:100%;">
    <br>
</p>

### Accessing web panel
Go to [http://localhost:8000](http://localhost:8000) in your web browser

![preview](https://raw.githubusercontent.com/juanhuttemann/nitr-agent/master/images/login-web.png)

Access with default **username** and **password**: **admin admin**

![preview](https://raw.githubusercontent.com/juanhuttemann/nitr-agent/master/images/panel-web.png)

## API

### Available Endpoints

These endpoints allow you to get system and hardware information about your host.

| Verb   | Endpoint                      | JSON Data                    |
|--------|-------------------------------|------------------------------|
|GET     | /cpu                          | [CPU](#cpu)                  |
|GET     | /bios                         | [Bios](#bios)                |
|GET     | /bandwidth                    | [Bandwidth](#bandwidth)      |
|GET     | /chassis                      | [Chassis](#chassis)          |
|GET     | /disks                        | [Disks](#disks)              |
|GET     | /drives                       | [Drives](#drives)            |
|GET     | /gpu                          | [GPU](#gpu)                  |
|GET     | /isp                          | [ISP](#isp)                  |
|GET     | /network                      | [Network](#network)          |
|GET     | /processes                    | [Processes](#processes)      |
|GET     | /ram                          | [RAM](#ram)                  |
|GET     | /baseboard                    | [Baseboard](#baseboard)      |
|GET     | /product                      | [Product](#product)          |


### How to Use

Call the above endpoints with ?key=yourapikey in the URL or pass the x-api-key header with your api key as value and you will get success response.

#### Examples:

- Requesting CPU Information.
>In the terminal.
```
curl -X Get 'http://localhost:8000/api/v1/cpu' -H 'x-api-key:yourapikeyhere'
```
>JSON Response:

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

- Requesting Host Information.

>In the web browser.

```
http://localhost:8000/api/v1/host?key=yourapikeyhere
```

![preview](https://raw.githubusercontent.com/juanhuttemann/nitr-agent/master/images/browser-api.png)

### JSON Data References

#### CPU 
*returns a json object*
| Key       | Data Type      | Description              |
|-----------|----------------|--------------------------|
| brand     | string         | CPU Brand                |
| cores     | integer        | Amount of CPU cores      |
| threads   | integer        | Amount of CPU threads    |
| usage     | float          | CPU usage percentage     |
| usageEach | Array of float | Usage percentage per CPU |


#### Bios
*returns a json object*

| Key       | Data Type      | Description              |
|-----------|----------------|--------------------------|
| vendor    | string         | Vendor                   |
| version   | string         | Bios version             |
| date      | string         | Bios last update         |


#### Bandwidth
*returns a json array of objects*
| Key       | Data Type      | Description              |
|-----------|----------------|--------------------------|
| name      | string         | Network Interface name   |
| rxBytes   | integer        | Amount of bytes received |
| txBytes   | integer        | Amount of bytes sent     |
| rxPackets | integer        | Total packets received   |
| txPackets | integer        | Total packets sent       |

#### Chassis
*returns a json object*
| Key       | Data Type      | Description              |
|-----------|----------------|--------------------------|
| type      | string         | Type                     |
| vendor    | string         | Chassis vendor           |
| serial    | string         | Chassis serial           |

#### Disks
*returns a json array of objects*
| Key        | Data Type       | Description                      |
|------------|-----------------|----------------------------------|
| mountPoint | string          | Drive Letter or Mount Point      |
| free       | integer         | Available disk space in bytes    |
| size       | integer         | Total disk space in bytes        |
| used       | integer         | Used disk space in bytes         |
| percent    | float           | Disk usage percent               |

#### Drives
*returns a json array of objects*
| Key        | Data Type       | Description                      |
|------------|-----------------|----------------------------------|
| name       | string          | Drive name                       |
| type       | string          | Drive type                       |
| model      | string          | Drive model                      |
| serial     | string          | Drive serial                     |

#### GPU
*returns a json object*
| Key       | Data Type      | Description              |
|-----------|----------------|--------------------------|
| brand     | string         | GPU Brand                |
| model     | string         | GPU Model                |

#### ISP
*returns a json object*
| Key       | Data Type      | Description              |
|-----------|----------------|--------------------------|
| isp       | string         | Internet Service Provider|
| ip        | string         | Public IP Address        |
| lat       | string         | Location Latitude        |
| lon       | string         | Location Longitude       |

#### Network
*returns a json array of objects*
| Key       | Data Type       | Description                            |
|-----------|-----------------|----------------------------------------|
| name      | string          | Network Interface name                 |
| addresses | Array of string | IPv4 and IPv6 list                     |
| mac       | string          | MAC Address                            |
| active    | boolean         | True if the Network Interface is Up    |


#### Processes
*returns a json array of objects*
| Key       | Data Type      | Description              |
|-----------|----------------|--------------------------|
| pid       | integer        | Process ID               |
| name      | string         | Process Name             |

#### Ram
*returns a json object*
| Key       | Data Type      | Description              |
|-----------|----------------|--------------------------|
| total     | integer        | Total RAM in bytes       |
| free      | integer        | Free RAM in bytes        |
| usage     | integer        | Used RAM in bytes        |

#### Baseboard
*returns a json object*
| Key       | Data Type      | Description              |
|-----------|----------------|--------------------------|
| vendor    | string         | Baseboard vendor         |
| assetTag  | string         | Asset Tag                |
| serial    | string         | Baseboard serial         |
| version   | string         | Baseboard Version        |

#### Product
*returns a json object*
| Key       | Data Type      | Description              |
|-----------|----------------|--------------------------|
| vendor    | string         | Product vendor           |
| family    | string         | Product family           |
| assetTag  | string         | Asset Tag                |
| serial    | string         | Product serial           |
| uuid      | string         | Product UUID             |
| sku       | string         | Product SKU              |
| version   | string         | Product Version          |

## Settings

The following settings are located in the `config.ini` file
  

### Server Port

By default, the web server starts on port 8000.


```

port: 3000

```

### Open Browser on Startup

If true, opens your default web browser on server startup.


```

open_browser_on_startup: true

```

### Enabling Logs

If true, logs are saved in `nitr.log` file, otherwise logs are printed out to console.


```

save_logs: true

```

### Enable SSL

If true, server starts using HTTPS protocol.  Certificate and Key must be provided
```
ssl_enabled: true
ssl_certificate: /path/to/file.crt
ssl_certificate_key: /path/to/file.key
```   

## Platform Support

**Windows**

Tested in Windows 10

**Linux**

Tested in:
- Ubuntu Linux 20.04 LTS
- Debian Linux 10


## Powered by

* [Fiber](https://gofiber.io/) - The web framework used
* [bbolt](https://github.com/etcd-io/bbolt) - Database
* [UIKit](https://getuikit.com/) - Front-End framework

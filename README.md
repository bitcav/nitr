﻿
<p align="center">
    <img alt="Nitr" height="125" src="https://raw.githubusercontent.com/bitcav/nitr/master/app/assets/images/logo.png" style="max-width:100%;">
    <br>
</p>

<div align="center">

![go](https://raw.githubusercontent.com/bitcav/nitr/master/images/goversion.svg) [![Build Status](https://travis-ci.org/bitcav/nitr.svg?branch=master)](https://travis-ci.org/bitcav/nitr) ![Release](https://raw.githubusercontent.com/bitcav/nitr/master/images/release.svg)  [![Go Report Card](https://goreportcard.com/badge/github.com/bitcav/nitr)](https://goreportcard.com/report/github.com/bitcav/nitr) [![MIT license](https://img.shields.io/badge/License-MIT-blue.svg)](https://github.com/bitcav/nitr/blob/master/LICENSE)

</div>

Nitr is a **cross-platform remote monitoring tool** written in Golang for **system information gathering**, making it available through a **JSON API**. 

The main purpose of this project is to provide highly available data of **CPU, RAM, Disks, Network, Processes** and so on, to make use of them in applications such as **web administration panels** or **mobile apps**. 

<p>
    <img alt="Nitr" src="https://raw.githubusercontent.com/bitcav/nitr/master/images/usage.gif" style="width:100%;">
</p>

> curl + jq demo

Table of contents
=================
   * [Installation](#gear-installation)
	    * [Download](#download)
	    *  [Build](#build)
   * [Running](#rocket-running)
   * [Docker](#whale-docker)
   * [Web Panel](#web-panel)
   * [Usage](#usage)
   * [Api v1](#api-v1)
	   * [Available Endpoints](#satellite-available-endpoints)
	   * [JSON Data References](#mag-json-data-references)
 * [Settings](#wrench-settings)
 * [Platform Support](#heavy_check_mark-platform-support)
 * [Powered by](#zap-powered-by)


   

## :gear: Installation

### Download

https://github.com/bitcav/nitr/releases/latest

### Building from source
Note: go version 1.13 or higher is required building it from the source.

#### Clone
```
git clone https://github.com/bitcav/nitr.git
```
#### Build
```
cd nitr
go build
```

## :rocket: Running

**Linux**
```
./nitr
```

**Windows**
You can double click the .exe file or type in cmd
```
nitr.exe
```
the server will start listening on port 8000 by default

<p style="width:100%;">
    <img alt="app" src="https://raw.githubusercontent.com/bitcav/nitr/master/images/app-start.gif">
    <br>
</p>

## :whale: Docker

Build image using command: 
```
docker build -t nitr . 
```

Run container:

```
docker run -d -p 8000:8000 nitr:latest
```

### Web Panel
Go to [http://localhost:8000](http://localhost:8000) in your web browser

![preview](https://raw.githubusercontent.com/bitcav/nitr/master/images/login-web.png)

Access with default **username** and **password**: **admin admin**

![preview](https://raw.githubusercontent.com/bitcav/nitr/master/images/panel-web.png)

## Usage

Call [the API endpoints](#available-endpoints) with ***?key=yourapikey*** in the URL or pass the ***x-api-key*** header with your api key as value and you will get success response.

### Examples:

- Requesting CPU Information.
>In the terminal.
```
curl -X Get 'http://localhost:8000/api/v1/cpu' -H 'x-api-key:yourapikeyhere'
```
>JSON Response:

```json
{
	"vendor":"GenuineIntel",
	"model":"Intel(R) Core(TM) i7-4810MQ CPU @ 2.80GHz",
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

![preview](https://raw.githubusercontent.com/bitcav/nitr/master/images/browser-api.png)


## API v1

### Root Endpoint

```
http://localhost:8000/api/v1
```

### :satellite: Available Endpoints

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
|GET     | /memory                       | [Memory](#Memory)            |





## :mag: JSON Data References

### CPU 
> JSON Object

| Key       | Data Type      | Description              |
|-----------|----------------|--------------------------|
| vendor    | string         | CPU Vendor               |
| model     | string         | CPU Model                |
| cores     | integer        | Amount of CPU cores      |
| threads   | integer        | Amount of CPU threads    |
| clockSpeed| float          | Clock Speed in Mhz       |
| usage     | float          | CPU usage percentage     |
| usageEach | Array of float | Usage percentage per CPU |


### Bios
> JSON Object

| Key       | Data Type      | Description              |
|-----------|----------------|--------------------------|
| vendor    | string         | Vendor                   |
| version   | string         | Bios version             |
| date      | string         | Bios last update         |


### Bandwidth
>JSON Array of Objects

| Key       | Data Type      | Description              |
|-----------|----------------|--------------------------|
| name      | string         | Network Interface name   |
| rxBytes   | integer        | Amount of bytes received |
| txBytes   | integer        | Amount of bytes sent     |
| rxPackets | integer        | Total packets received   |
| txPackets | integer        | Total packets sent       |

### Chassis
:lock: Requires running **nitr** with elevated privileges 
> JSON Object

| Key       | Data Type      | Description              |
|-----------|----------------|--------------------------|
| type      | string         | Type                     |
| vendor    | string         | Chassis vendor           |
| serial    | string         | Chassis serial           |

### Disks
>JSON Array of Objects

| Key        | Data Type       | Description                      |
|------------|-----------------|----------------------------------|
| mountPoint | string          | Drive Letter or Mount Point      |
| free       | integer         | Available disk space in bytes    |
| size       | integer         | Total disk space in bytes        |
| used       | integer         | Used disk space in bytes         |
| percent    | float           | Disk usage percent               |

### Drives
> JSON Array of Objects

| Key        | Data Type       | Description                      |
|------------|-----------------|----------------------------------|
| name       | string          | Drive name                       |
| type       | string          | Drive type                       |
| model      | string          | Drive model                      |
| serial     | string          | Drive serial                     |

### GPU
> JSON Array of Objects

| Key       | Data Type      | Description              |
|-----------|----------------|--------------------------|
| brand     | string         | GPU Brand                |
| model     | string         | GPU Model                |

### ISP
>JSON Object

| Key       | Data Type      | Description              |
|-----------|----------------|--------------------------|
| isp       | string         | Internet Service Provider|
| ip        | string         | Public IP Address        |
| lat       | string         | Location Latitude        |
| lon       | string         | Location Longitude       |

### Network
>JSON Array of Objects

| Key       | Data Type       | Description                            |
|-----------|-----------------|----------------------------------------|
| name      | string          | Network Interface name                 |
| addresses | Array of string | IPv4 and IPv6 list                     |
| mac       | string          | MAC Address                            |
| active    | boolean         | True if the Network Interface is Up    |


### Processes
> JSON Array of Objects

| Key       | Data Type      | Description              |
|-----------|----------------|--------------------------|
| pid       | integer        | Process ID               |
| name      | string         | Process Name             |

### Ram
> JSON Object

| Key       | Data Type      | Description              |
|-----------|----------------|--------------------------|
| total     | integer        | Total RAM in bytes       |
| free      | integer        | Free RAM in bytes        |
| usage     | integer        | Used RAM in bytes        |

### Baseboard
:lock: Requires running **nitr** with elevated privileges 
> JSON Object

| Key       | Data Type      | Description              |
|-----------|----------------|--------------------------|
| vendor    | string         | Baseboard vendor         |
| assetTag  | string         | Asset Tag                |
| serial    | string         | Baseboard serial         |
| version   | string         | Baseboard Version        |

### Product
:lock: Requires running **nitr** with elevated privileges 
>JSON Object

| Key       | Data Type      | Description              |
|-----------|----------------|--------------------------|
| vendor    | string         | Product vendor           |
| family    | string         | Product family           |
| assetTag  | string         | Asset Tag                |
| serial    | string         | Product serial           |
| uuid      | string         | Product UUID             |
| sku       | string         | Product SKU              |
| version   | string         | Product Version          |

### Memory
:lock: Requires running **nitr** with elevated privileges 
>JSON Array of Objects

| Key          | Data Type       | Description                     |
|--------------|-----------------|---------------------------------|
| bank		   | string 		 | Bank Identifier                 |
| size         | integer         | Size                            |
| unit         | string          | Unit (KB or MB)                 |
| type         | string          | Type                            |
| formFactor   | string          | Form Factor                     |
| manufacturer | string          | Manufacturer                    |
| serial       | string          | Serial Number                   |
| assetTag     | string          | Asset Tag                       |
| partNumber   | string          | Part Number                     |
| speed        | integer         | Speed in MT/s                   |
| dataWidth    | integer         | Data Width in bits              |
| totalWidth   | integer         | Total Data Width in bits        |


## :wrench: Settings

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

## :heavy_check_mark: Platform Support

**Windows**

Tested:
- Windows 10
- Windows 7 SP1

**Linux**

Tested:
- Ubuntu Linux 20.04 LTS
- Debian Linux 10
- Manjaro Linux 20


## :zap: Powered by

* [Fiber](https://gofiber.io/) - The web framework used
* [bbolt](https://github.com/etcd-io/bbolt) - Database
* [UIKit](https://getuikit.com/) - Front-End framework
* [gopsutil](https://github.com/shirou/gopsutil) - psutil for Golang
* [ghw](https://github.com/jaypipes/ghw) - Golang HardWare discovery/inspection library
* [go-smbios](https://github.com/digitalocean/go-smbios) - Detection and access to System Management BIOS

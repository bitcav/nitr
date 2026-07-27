
<p align="center">
    <img alt="Nitr" height="125" src="https://raw.githubusercontent.com/bitcav/nitr/master/images/logo.png" style="max-width:100%;">
    <br>
</p>

<div align="center">

![Go version](https://raw.githubusercontent.com/bitcav/nitr/master/images/goversion.svg) [![CI](https://github.com/bitcav/nitr/actions/workflows/ci.yml/badge.svg)](https://github.com/bitcav/nitr/actions/workflows/ci.yml) ![Release](https://raw.githubusercontent.com/bitcav/nitr/master/images/release.svg) ![Go report](https://raw.githubusercontent.com/bitcav/nitr/master/images/goreport.svg) [![MIT license](https://img.shields.io/badge/License-MIT-blue.svg)](https://github.com/bitcav/nitr/blob/master/LICENSE) ![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20Windows-blue.svg)

</div>

# Nitr

A **cross-platform remote monitoring tool** written in Go that exposes **system and hardware information** (CPU, RAM, disks, network, processes, and more) through a **JSON API**, so it can be consumed by web admin panels, mobile apps, or anything that speaks HTTP.

<p>
    <img alt="Nitr" src="https://raw.githubusercontent.com/bitcav/nitr/master/images/usage.gif" style="width:100%;">
</p>

> curl + jq demo

**Try it in 30 seconds:**

```bash
# Linux (amd64) — see Installation for Windows / 32-bit
curl -L https://github.com/bitcav/nitr/releases/latest/download/nitr_linux_amd64 -o nitr
sudo install -m 755 nitr /usr/local/bin/
nitr            # start the server (default port 8000, default password 123456)
nitr key        # print your API key (needs the password)
```

```bash
curl -X GET http://localhost:8000/api/v1/cpu -H 'x-api-key: yourapikeyhere'
```

See [Usage](#usage) for the full endpoint list and response shapes.

## Table of contents

- [Installation](#installation)
  - [Quick install](#quick-install)
  - [Download](#download)
  - [Build](#build)
- [Running](#running)
- [Commands](#commands)
- [Docker](#docker)
- [Web panel](#web-panel)
- [QR Code](#qr-code)
- [Usage](#usage)
  - [Example](#example)
- [API v1](#api-v1)
  - [Available endpoints](#available-endpoints)
  - [JSON data references](#json-data-references)
- [Settings](#settings)
- [Platform support](#platform-support)
- [Powered by](#powered-by)

## Installation

### Quick install

**Linux (amd64)**
```
curl -L https://github.com/bitcav/nitr/releases/latest/download/nitr_linux_amd64 -o nitr
sudo install -m 755 nitr /usr/local/bin/
```

**Windows (amd64) — PowerShell**
```
Invoke-WebRequest https://github.com/bitcav/nitr/releases/latest/download/nitr_windows_amd64.exe -OutFile nitr.exe
```

> On 32-bit systems, swap the asset name for `nitr_linux_386` or `nitr_windows_386.exe`.

### Download

https://github.com/bitcav/nitr/releases/latest

### Build

Note: go version 1.26 or higher is required building it from the source.

#### Clone
```
git clone https://github.com/bitcav/nitr.git
```
#### Build
```
cd nitr
go build
```

## Running

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

## Commands

Help:

```
nitr -h
```

Print the version:

```
nitr version
```

Change password:

```
nitr passwd
```

Get API key:

```
nitr key
```

Print QR code:

```
nitr qr
```

## Docker

Build image using command:
```
docker build -t nitr . 
```

Run container:
```
docker run -d -p 8000:8000 nitr:latest
```

## Web panel

Go to [http://localhost:8000](http://localhost:8000) in your web browser

![preview](https://raw.githubusercontent.com/bitcav/nitr/master/images/login-web.png)

Access with default **password**: **123456**

![preview](https://raw.githubusercontent.com/bitcav/nitr/master/images/panel-web.png)

## QR Code

The QR Code contains the exact same information displayed in the Host Info Panel formatted as JSON.

## Usage

Request system info with an HTTP `GET` to [one of the API endpoints](#available-endpoints), passing the `x-api-key` header with your ***API key*** as the value, and you will get a success response.

### Example

Requesting CPU information.

> With curl.
```
curl -X GET 'http://localhost:8000/api/v1/cpu' -H 'x-api-key:yourapikeyhere'
```
> With PowerShell.
```
Invoke-RestMethod -Uri http://localhost:8000/api/v1/cpu -H @{"x-api-key"="yourapikeyhere"}
```
> JSON response:

```json
{
	"vendor":"GenuineIntel",
	"model":"Intel(R) Core(TM) i7-4810MQ CPU @ 2.80GHz",
	"cores":4,
	"threads":8,
	"clockSpeed":3800,
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

## API v1

### Root endpoint

```
http://localhost:8000/api/v1
```

### Available endpoints

These endpoints return system and hardware information about your **host**. Check the [example](#example) for a better understanding.

| Verb | Endpoint   | JSON Data               |
|------|------------|-------------------------|
| GET  | /cpu       | [CPU](#cpu)             |
| GET  | /bios      | [Bios](#bios)           |
| GET  | /bandwidth | [Bandwidth](#bandwidth) |
| GET  | /chassis   | [Chassis](#chassis)     |
| GET  | /disks     | [Disks](#disks)         |
| GET  | /drives    | [Drives](#drives)       |
| GET  | /devices   | [Devices](#devices)     |
| GET  | /gpu       | [GPU](#gpu)             |
| GET  | /host      | [Host](#host)           |
| GET  | /isp       | [ISP](#isp)             |
| GET  | /network   | [Network](#network)     |
| GET  | /processes | [Processes](#processes) |
| GET  | /ram       | [RAM](#ram)             |
| GET  | /baseboard | [Baseboard](#baseboard) |
| GET  | /product   | [Product](#product)     |
| GET  | /memory    | [Memory](#memory)       |

### JSON data references

<details>
<summary>CPU</summary>

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

</details>

<details>
<summary>Bios</summary>

### Bios

> JSON Object

| Key       | Data Type      | Description              |
|-----------|----------------|--------------------------|
| vendor    | string         | Vendor                   |
| version   | string         | Bios version             |
| date      | string         | Bios last update         |

</details>

<details>
<summary>Bandwidth</summary>

### Bandwidth

>JSON Array of Objects

| Key       | Data Type      | Description              |
|-----------|----------------|--------------------------|
| name      | string         | Network Interface name   |
| rxBytes   | integer        | Amount of bytes received |
| txBytes   | integer        | Amount of bytes sent     |
| rxPackets | integer        | Total packets received   |
| txPackets | integer        | Total packets sent       |

</details>

<details>
<summary>Chassis</summary>

### Chassis

:lock: Requires running **nitr** with elevated privileges 
> JSON Object

| Key       | Data Type      | Description              |
|-----------|----------------|--------------------------|
| type      | string         | Type                     |
| vendor    | string         | Chassis vendor           |
| serial    | string         | Chassis serial           |

</details>

<details>
<summary>Disks</summary>

### Disks

>JSON Array of Objects

| Key        | Data Type       | Description                      |
|------------|-----------------|----------------------------------|
| mountPoint | string          | Drive Letter or Mount Point      |
| free       | integer         | Available disk space in bytes    |
| size       | integer         | Total disk space in bytes        |
| used       | integer         | Used disk space in bytes         |
| percent    | float           | Disk usage percent               |

</details>

<details>
<summary>Drives</summary>

### Drives

> JSON Array of Objects

| Key        | Data Type       | Description                      |
|------------|-----------------|----------------------------------|
| name       | string          | Drive name                       |
| type       | string          | Drive type                       |
| model      | string          | Drive model                      |
| serial     | string          | Drive serial                     |

</details>

<details>
<summary>Devices</summary>

### Devices

> JSON Array of Objects

| Key     | Data Type | Description          |
|---------|-----------|----------------------|
| product | string    | Device product name  |
| vendor  | string    | Device vendor        |
| address | string    | PCI address          |

</details>

<details>
<summary>GPU</summary>

### GPU

> JSON Array of Objects

| Key       | Data Type      | Description              |
|-----------|----------------|--------------------------|
| brand     | string         | GPU Brand                |
| model     | string         | GPU Model                |

</details>

<details>
<summary>Host</summary>

### Host

> JSON Object

| Key      | Data Type | Description          |
|----------|-----------|----------------------|
| name     | string    | Hostname             |
| os       | string    | Operating system     |
| platform | string    | Platform and version |
| arch     | string    | Architecture         |
| uptime   | integer   | Uptime in seconds    |

</details>

<details>
<summary>ISP</summary>

### ISP

>JSON Object

| Key       | Data Type      | Description              |
|-----------|----------------|--------------------------|
| isp       | string         | Internet Service Provider|
| ip        | string         | Public IP Address        |
| lat       | string         | Location Latitude        |
| lon       | string         | Location Longitude       |

</details>

<details>
<summary>Network</summary>

### Network

>JSON Array of Objects

| Key       | Data Type       | Description                            |
|-----------|-----------------|----------------------------------------|
| name      | string          | Network Interface name                 |
| addresses | Array of string | IPv4 and IPv6 list                     |
| mac       | string          | MAC Address                            |
| active    | boolean         | True if the Network Interface is Up    |

</details>

<details>
<summary>Processes</summary>

### Processes

> JSON Array of Objects

| Key       | Data Type      | Description              |
|-----------|----------------|--------------------------|
| pid       | integer        | Process ID               |
| name      | string         | Process Name             |

</details>

<details>
<summary>Ram</summary>

### Ram

> JSON Object

| Key       | Data Type      | Description              |
|-----------|----------------|--------------------------|
| total     | integer        | Total RAM in bytes       |
| free      | integer        | Free RAM in bytes        |
| usage     | integer        | Used RAM in bytes        |

</details>

<details>
<summary>Baseboard</summary>

### Baseboard

:lock: Requires running **nitr** with elevated privileges 
> JSON Object

| Key       | Data Type      | Description              |
|-----------|----------------|--------------------------|
| vendor    | string         | Baseboard vendor         |
| assetTag  | string         | Asset Tag                |
| serial    | string         | Baseboard serial         |
| version   | string         | Baseboard Version        |

</details>

<details>
<summary>Product</summary>

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

</details>

<details>
<summary>Memory</summary>

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

</details>

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

## Platform support

Nitr publishes release binaries for **Linux** and **Windows**, on **amd64** and **386** (32-bit) architectures:

| OS      | amd64 | 386 |
|---------|-------|-----|
| Linux   | yes   | yes |
| Windows | yes   | yes |

Download them from the [latest release](https://github.com/bitcav/nitr/releases/latest).

## Powered by

* [Fiber](https://gofiber.io/) - The web framework used
* [bbolt](https://github.com/etcd-io/bbolt) - Database
* [UIKit](https://getuikit.com/) - Front-End framework
* [gopsutil](https://github.com/shirou/gopsutil) - psutil for Golang
* [ghw](https://github.com/jaypipes/ghw) - Golang HardWare discovery/inspection library
* [go-smbios](https://github.com/digitalocean/go-smbios) - Detection and access to System Management BIOS

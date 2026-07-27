
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
- [Prometheus metrics](#prometheus-metrics)
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

> `nitr passwd`, `nitr key`, and `nitr qr` operate on the database in the **current working directory** and need the server to have been started there first — running the server is the only thing that creates `nitr.db`. Without it they refuse and exit 1, creating nothing:
> ```
> Error: no nitr database found in /path/to/dir — start the nitr server in this directory first
> ```

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

## Prometheus metrics

Nitr exposes a `/metrics` endpoint emitting the standard Prometheus exposition format, so Nitr becomes a drop-in scrape target for Grafana, Alertmanager, VictoriaMetrics, and any other monitoring stack that speaks Prometheus.

The endpoint sits behind the **same `x-api-key` header** as the JSON API — the exposed data is hardware detail and should not be public. Prometheus passes custom headers through `http_headers` in `scrape_config` (`secrets:` rather than `values:` so the key is redacted on Prometheus's own config page):

```yaml
scrape_configs:
  - job_name: nitr
    scheme: http
    metrics_path: /metrics
    static_configs:
      - targets: ["localhost:8000"]
    http_headers:
      x-api-key:
        secrets: [yourapikeyhere]
```

### Exposed metrics

All metrics use the `nitr_` prefix, base units (seconds, bytes), and `snake_case`. CPU time is a counter (cumulative); the rest are gauges reflecting host state at scrape time.

| Metric                       | Type      | Labels        | Description                                                     |
|------------------------------|-----------|---------------|-----------------------------------------------------------------|
| `nitr_cpu_seconds_total`     | counter   | `cpu`, `mode` | Cumulative CPU seconds per core and mode (user, system, idle...) |
| `nitr_ram_total_bytes`       | gauge     |               | Total RAM in bytes                                              |
| `nitr_ram_free_bytes`        | gauge     |               | Free RAM in bytes                                               |
| `nitr_ram_used_bytes`        | gauge     |               | Used RAM in bytes                                               |
| `nitr_disk_free_bytes`       | gauge     | `mountpoint`  | Free disk space in bytes                                        |
| `nitr_disk_size_bytes`       | gauge     | `mountpoint`  | Total disk size in bytes                                        |
| `nitr_disk_used_bytes`       | gauge     | `mountpoint`  | Used disk space in bytes                                        |

CPU usage is not a separate metric; derive it from the counter with PromQL, e.g. average busy fraction per core over 5m:

```promql
1 - avg(rate(nitr_cpu_seconds_total{mode="idle"}[5m])) by (cpu)
```

Per-interface bandwidth is **not** exposed yet. `bandwidth.Info` derives a per-second delta by reading netdev counters twice with a 1s sleep, and a blocking scrape endpoint causes Prometheus scrape timeouts. It will be added once a background sampler lands.

Example scrape with curl:

```bash
curl http://localhost:8000/metrics -H 'x-api-key: yourapikeyhere'
```

```text
# HELP nitr_cpu_seconds_total Cumulative seconds the CPU has spent in each mode, per core. Counter; derive utilisation with avg(rate(nitr_cpu_seconds_total[5m])) by (mode).
# TYPE nitr_cpu_seconds_total counter
nitr_cpu_seconds_total{cpu="0",mode="idle"} 1.2345678e+06
nitr_cpu_seconds_total{cpu="0",mode="user"} 3.4567890e+05
# HELP nitr_disk_free_bytes Free disk space in bytes.
# TYPE nitr_disk_free_bytes gauge
nitr_disk_free_bytes{mountpoint="/"} 1.2345678e+07
# HELP nitr_ram_total_bytes Total RAM in bytes.
# TYPE nitr_ram_total_bytes gauge
nitr_ram_total_bytes 8.3319750656e+09
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

| Key       | Data Type        | Description                            |
|-----------|------------------|----------------------------------------|
| name      | string           | Network Interface name                 |
| addresses | Array of objects | IPv4 and IPv6 addresses (see example)  |
| mac       | string           | MAC Address                            |
| active    | boolean          | True if the Network Interface is Up    |

Each entry of `addresses` is an object with a single `ip` key.

Example response:

```json
[
  {
    "name": "eth0",
    "addresses": [
      { "ip": "192.168.1.10" },
      { "ip": "fe80::1" }
    ],
    "mac": "00:11:22:33:44:55",
    "active": true
  }
]
```

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

Every push runs the full test suite on two CI runners — `ubuntu-latest` and `windows-2025` — and on each leg additionally builds the real binary and runs `nitr version`, asserting it exits 0 and prints a sane version string. The 386 builds in the table above are cross-compiled by CI but are not executed there.

## Powered by

* [Fiber](https://gofiber.io/) - The web framework used
* [bbolt](https://github.com/etcd-io/bbolt) - Database
* [UIKit](https://getuikit.com/) - Front-End framework
* [gopsutil](https://github.com/shirou/gopsutil) - psutil for Golang
* [ghw](https://github.com/jaypipes/ghw) - Golang HardWare discovery/inspection library
* [go-smbios](https://github.com/digitalocean/go-smbios) - Detection and access to System Management BIOS

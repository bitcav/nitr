# API reference

Per-endpoint field tables for the Nitr JSON API. The endpoint table lives in the [README](../README.md#available-endpoints).

A `:lock:` at the top of a section means the whole endpoint needs root. A `:lock:` note under a table means only that field is restricted — on Linux, privilege is enforced per field by the file mode of `/sys/class/dmi/id/*` (see `drivers/firmware/dmi-id.c` in the kernel), not per endpoint.

## Overview

`GET /api/v1/` — a lightweight summary combining the other endpoints.

> JSON Object

```json
{
  "host": { "...": "see Host" },
  "cpuUsage": 8.35,
  "ram": { "...": "see RAM" }
}
```

`host` is the [Host](#host) object, `ram` is the [RAM](#ram) object, and `cpuUsage` is a float (CPU usage percentage).

## CPU

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

## Bios

> JSON Object

| Key       | Data Type      | Description              |
|-----------|----------------|--------------------------|
| vendor    | string         | Vendor                   |
| version   | string         | Bios version             |
| date      | string         | Bios last update         |

## Bandwidth

>JSON Array of Objects

| Key       | Data Type      | Description              |
|-----------|----------------|--------------------------|
| name      | string         | Network Interface name   |
| rxBytes   | integer        | Amount of bytes received |
| txBytes   | integer        | Amount of bytes sent     |
| rxPackets | integer        | Total packets received   |
| txPackets | integer        | Total packets sent       |

## Chassis

> JSON Object

| Key       | Data Type      | Description              |
|-----------|----------------|--------------------------|
| type      | string         | Type                     |
| vendor    | string         | Chassis vendor           |
| serial    | string         | Chassis serial           |

> :lock: `serial` requires root. `/sys/class/dmi/id/chassis_serial` is mode `0400`; `chassis_type` and `chassis_vendor` are `0444` (world-readable). Without root, `serial` comes back as `unknown`.

## Disks

>JSON Array of Objects

| Key        | Data Type       | Description                      |
|------------|-----------------|----------------------------------|
| mountPoint | string          | Drive Letter or Mount Point      |
| free       | integer         | Available disk space in bytes    |
| size       | integer         | Total disk space in bytes        |
| used       | integer         | Used disk space in bytes         |
| percent    | float           | Disk usage percent               |

## Drives

> JSON Array of Objects

| Key        | Data Type       | Description                      |
|------------|-----------------|----------------------------------|
| name       | string          | Drive name                       |
| type       | string          | Drive type                       |
| model      | string          | Drive model                      |
| serial     | string          | Drive serial                     |

## Devices

> JSON Array of Objects

| Key     | Data Type | Description          |
|---------|-----------|----------------------|
| product | string    | Device product name  |
| vendor  | string    | Device vendor        |
| address | string    | PCI address          |

## GPU

> JSON Array of Objects

| Key       | Data Type      | Description              |
|-----------|----------------|--------------------------|
| brand     | string         | GPU Brand                |
| model     | string         | GPU Model                |

## Host

> JSON Object

| Key      | Data Type | Description          |
|----------|-----------|----------------------|
| name     | string    | Hostname             |
| os       | string    | Operating system     |
| platform | string    | Platform and version |
| arch     | string    | Architecture         |
| uptime   | integer   | Uptime in seconds    |

## ISP

>JSON Object

| Key       | Data Type      | Description              |
|-----------|----------------|--------------------------|
| isp       | string         | Internet Service Provider|
| ip        | string         | Public IP Address        |
| lat       | string         | Location Latitude        |
| lon       | string         | Location Longitude       |

## Network

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

## Processes

> JSON Array of Objects

| Key       | Data Type      | Description              |
|-----------|----------------|--------------------------|
| pid       | integer        | Process ID               |
| name      | string         | Process Name             |

## Ram

> JSON Object

| Key       | Data Type      | Description              |
|-----------|----------------|--------------------------|
| total     | integer        | Total RAM in bytes       |
| free      | integer        | Free RAM in bytes        |
| usage     | integer        | Used RAM in bytes        |

## Baseboard

> JSON Object

| Key       | Data Type      | Description              |
|-----------|----------------|--------------------------|
| vendor    | string         | Baseboard vendor         |
| assetTag  | string         | Asset Tag                |
| serial    | string         | Baseboard serial         |
| version   | string         | Baseboard Version        |

> :lock: `serial` requires root. `/sys/class/dmi/id/board_serial` is mode `0400`; the other Baseboard fields (`board_vendor`, `board_asset_tag`, `board_version`) are `0444`.

## Product

>JSON Object

| Key       | Data Type      | Description                              |
|-----------|----------------|------------------------------------------|
| vendor    | string         | Product vendor                           |
| family    | string         | Product family                           |
| familiy   | string         | Product family (deprecated; misspelled) |
| name      | string         | Product name                             |
| serial    | string         | Product serial                           |
| uuid      | string         | Product UUID                             |
| sku       | string         | Product SKU                              |
| version   | string         | Product version                          |

> `familiy` is a misspelled duplicate of `family`, retained for one release as
> a compatibility shim and **will be removed** in a later breaking change; read
> `family` instead. This endpoint has **no `assetTag` key**: the value it
> previously emitted under `assetTag` carried the product `name`, not an asset
> tag. The machine's real asset tag is served at [`/baseboard`](#baseboard)
> (the `assetTag` field), populated from `ghw.Baseboard().AssetTag`.

> :lock: `serial` and `uuid` require root. `/sys/class/dmi/id/product_serial` and `product_uuid` are mode `0400`; the other Product fields (`sys_vendor`, `product_family`, `product_name`, `product_sku`, `product_version`) are `0444`.

## Memory

:lock: Requires running **nitr** with elevated privileges

>JSON Array of Objects

| Key          | Data Type       | Description                     |
|--------------|-----------------|---------------------------------|
| bank         | string          | Bank Identifier                 |
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

> Unlike the other `:lock:` endpoints, `/memory` does not degrade per field: `go-memdev` reads the raw SMBIOS table, which on Linux needs root. On failure the handler returns an error response instead of swallowing it. The status code is `403` when the underlying error is a permission error (e.g. running non-root and `/dev/mem` / `/sys/firmware/dmi/tables/*` is unreadable), and `500` otherwise. The body is the standard error envelope (`{"message": "...", "status": <code>}`), so callers can distinguish "needs root" from "broken".

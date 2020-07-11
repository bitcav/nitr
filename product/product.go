package product

import (
	"fmt"

	"github.com/jaypipes/ghw"
)

type Product struct {
	Vendor       string `json:"vendor"`
	Family       string `json:"familiy"`
	Name         string `json:"assetTag"`
	SerialNumber string `json:"serial"`
	UUID         string `json:"uuid"`
	SKU          string `json:"sku"`
	Version      string `json:"version"`
}

//Info returns Product struct containing product information
func Info() Product {
	product, err := ghw.Product()
	if err != nil {
		fmt.Printf("Error getting product info: %v", err)
	}
	return Product{
		Family:       product.Family,
		Name:         product.Name,
		SerialNumber: product.SerialNumber,
		UUID:         product.UUID,
		SKU:          product.SKU,
		Vendor:       product.Vendor,
		Version:      product.Version,
	}
}

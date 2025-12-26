package main

import "encoding/xml"

type Argument struct {
	XMLName   xml.Name `xml:"arg"`
	Name      string   `xml:"name,attr"`
	Type      string   `xml:"type,attr"`
	Interface string   `xml:"interface,attr"`
}

type Request struct {
	XMLName   xml.Name   `xml:"request"`
	Name      string     `xml:"name,attr"`
	Arguments []Argument `xml:"arg"`
}

type Event struct {
	XMLName   xml.Name   `xml:"event"`
	Name      string     `xml:"name,attr"`
	Arguments []Argument `xml:"arg"`
}

type Enum struct {
	XMLName xml.Name `xml:"enum"`
	Name    string   `xml:"name,attr"`
	Entries []struct {
		XMLName xml.Name `xml:"entry"`
		Name    string   `xml:"name,attr"`
		Value   string   `xml:"value,attr"`
	} `xml:"entry"`
}

type Protocol struct {
	XMLName    xml.Name `xml:"protocol"`
	Name       string   `xml:"name,attr"`
	Interfaces []struct {
		XMLName  xml.Name  `xml:"interface"`
		Name     string    `xml:"name,attr"`
		Version  int       `xml:"version,attr"`
		Requests []Request `xml:"request"`
		Events   []Event   `xml:"event"`
		Enums    []Enum    `xml:"enum"`
	} `xml:"interface"`
}

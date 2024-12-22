package main

import "net"

type Ip4 struct {
	ID    int    `json:"id"`
	Saved bool   `json:"saved"`
	Value string `json:"value"`
}

func (i *Ip4) IsValid() bool {
	return net.ParseIP(i.Value) != nil
}

type MD5 struct {
	Value string `json:"value"`
	Saved bool   `json:"saved"`
}

func (m *MD5) IsValid() bool {
	return len(m.Value) == 32
}

type SHA1 struct {
	Value string `json:"value"`
	Saved bool   `json:"saved"`
}

func (s *SHA1) IsValid() bool {
	return len(s.Value) == 40
}

type SHA256 struct {
	Value string `json:"value"`
	Saved bool   `json:"saved"`
}

func (s *SHA256) IsValid() bool {
	return len(s.Value) == 64
}

type SHA512 struct {
	Value string `json:"value"`
	Saved bool   `json:"saved"`
}

func (s *SHA512) IsValid() bool {
	return len(s.Value) == 128
}

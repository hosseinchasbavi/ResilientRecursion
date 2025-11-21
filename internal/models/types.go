package models

type Request struct {
    R float64 `json:"r"`
    N int     `json:"n"`
}

type Response struct {
    R      float64 `json:"r"`
    N      int     `json:"n"`
    Result float64 `json:"result"`
}
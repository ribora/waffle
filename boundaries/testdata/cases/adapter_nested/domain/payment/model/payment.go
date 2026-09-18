package model

type Method struct {
	Id    string
	Brand string
}

type Payment struct {
	Id     int
	Method Method
}

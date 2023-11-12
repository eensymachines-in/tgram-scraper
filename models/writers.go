package models

type ResponseWriter interface {
	Write(using interface{}) (map[string]interface{}, error)
}

type HttpResponseWriter struct {
}

func (hpw *HttpResponseWriter) Write() (map[string]interface{}, error) {
	return nil, nil
}

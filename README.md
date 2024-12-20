# Portech MV-370 Go API client

This package provides a simple way to send and receive SMS messages on a Portech MV-370 GSM/SIP Gateway.

## Minimum example
```golang
c, err := mv370.New(host, username, password, nil)
if err != nil {
    panic(err)
}
err = c.SendSMS(tel, text)
if err != nil {
    panic(err)
}
```
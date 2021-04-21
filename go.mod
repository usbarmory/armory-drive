module github.com/f-secure-foundry/armory-drive

go 1.16

require (
	github.com/f-secure-foundry/armory-boot v0.0.0-20210412183525-0b1c4de61b0e
	github.com/f-secure-foundry/crucible v0.0.0-20210412160519-6e04f63398d9
	github.com/f-secure-foundry/hid v0.0.0-20210318233634-85ced88a1ffe
	github.com/f-secure-foundry/tamago v0.0.0-20210412174520-306e76ef8c31
	github.com/google/go-github/v34 v34.0.0
	github.com/mitchellh/go-fs v0.0.0-20180402235330-b7b9ca407fff
	github.com/skip2/go-qrcode v0.0.0-20200617195104-da1b6568686e
	golang.org/x/crypto v0.0.0-20210322153248-0c34fe9e7dc2
	golang.org/x/oauth2 v0.0.0-20180821212333-d2e6202438be
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/protobuf v1.26.0
)

replace gvisor.dev/gvisor => github.com/f-secure-foundry/gvisor v0.0.0-20210201110150-c18d73317e0f

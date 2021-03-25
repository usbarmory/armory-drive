module github.com/f-secure-foundry/armory-drive

go 1.15

require (
	github.com/f-secure-foundry/crucible v0.0.0-20210322082828-1a26aae60e6c // indirect
	github.com/f-secure-foundry/tamago v0.0.0-20210217103808-875e533027e3
	github.com/google/go-cmp v0.5.3-0.20201020212313-ab46b8bd0abd // indirect
	github.com/mitchellh/go-fs v0.0.0-20180402235330-b7b9ca407fff
	github.com/skip2/go-qrcode v0.0.0-20200617195104-da1b6568686e
	golang.org/x/crypto v0.0.0-20201221181555-eec23a3978ad
	golang.org/x/sync v0.0.0-20201207232520-09787c993a3a
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/protobuf v1.25.1-0.20201020201750-d3470999428b
)

replace gvisor.dev/gvisor => github.com/f-secure-foundry/gvisor v0.0.0-20210201110150-c18d73317e0f

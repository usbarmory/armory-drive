module github.com/f-secure-foundry/armory-drive

go 1.15

require (
	github.com/f-secure-foundry/tamago v0.0.0-20210208163511-6cc9699b14bd
	github.com/golang/protobuf v1.4.1
	github.com/mitchellh/go-fs v0.0.0-20180402235330-b7b9ca407fff
	github.com/skip2/go-qrcode v0.0.0-20200617195104-da1b6568686e
	golang.org/x/crypto v0.0.0-20201221181555-eec23a3978ad
	golang.org/x/sync v0.0.0-20201207232520-09787c993a3a
	golang.org/x/sys v0.0.0-20210124154548-22da62e12c0c // indirect
	golang.org/x/term v0.0.0-20201210144234-2321bbc49cbf
	golang.org/x/time v0.0.0-20201208040808-7e3f01d25324 // indirect
	google.golang.org/protobuf v1.25.1-0.20201020201750-d3470999428b
	gvisor.dev/gvisor v0.0.0-20210211014518-81ea0016e623
)

replace gvisor.dev/gvisor => github.com/f-secure-foundry/gvisor v0.0.0-20210201110150-c18d73317e0f

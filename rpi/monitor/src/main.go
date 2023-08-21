package main

import (
	"flag"
	"image"
	"image/draw"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
	"unicode/utf8"

	"periph.io/x/conn/v3/display"
	"periph.io/x/conn/v3/i2c"
	"periph.io/x/conn/v3/i2c/i2creg"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/devices/v3/ssd1306"
	"periph.io/x/devices/v3/ssd1306/image1bit"
	"periph.io/x/host/v3"
)

type Args struct {
	i2cID *string
	inet *string
	hz physic.Frequency
	height *int
	width *int
	rotated *bool
	sequential *bool
	swapTopBottom *bool
}

type Info struct {
	hostname string
	ipv4 string
}

var info Info
var args Args

func getHostname() string {
	name, err := os.Hostname()
	if err != nil {
		log.Printf("error geting hostname: %s", err)
		name = ""
	} else {
		log.Printf("Hostname: %s", name)
	}
	return name
}

func GetInternalIP(inet string) string {
	itf, _ := net.InterfaceByName(inet)
	item, _ := itf.Addrs()
	var ip net.IP
	for _, addr := range item {
			switch v := addr.(type) {
			case *net.IPNet:
					if !v.IP.IsLoopback() {
							if v.IP.To4() != nil {
									ip = v.IP
							}
					}
			}
	}
	if ip != nil {
			return ip.String()
	} else {
			return ""
	}
}

// resize is a simple but fast nearest neighbor implementation.
func resize(src image.Image, size image.Point) *image.NRGBA {
	srcMax := src.Bounds().Max
	dst := image.NewNRGBA(image.Rectangle{Max: size})
	for y := 0; y < size.Y; y++ {
		sY := (y*srcMax.Y + size.Y/2) / size.Y
		for x := 0; x < size.X; x++ {
			dst.Set(x, y, src.At((x*srcMax.X+size.X/2)/size.X, sY))
		}
	}
	return dst
}

func convert(disp display.Drawer, src image.Image) *image1bit.VerticalLSB {
	screenBounds := disp.Bounds()
	size := screenBounds.Size()
	src = resize(src, size)
	img := image1bit.NewVerticalLSB(screenBounds)
	r := src.Bounds()
	r = r.Add(image.Point{(size.X - r.Max.X) / 2, (size.Y - r.Max.Y) / 2})
	draw.Draw(img, r, src, image.Point{}, draw.Src)
	return img
}

// drawTextBottomRight draws text at the bottom right of img.
func drawInfo(img draw.Image, text string) {
	advance := utf8.RuneCountInString(text) * 7
	bounds := img.Bounds()
	if advance > bounds.Dx() {
		advance = 0
	} else {
		advance = bounds.Dx() - advance
	}
	drawText(img, image.Point{advance, bounds.Dy() - 1 - 13}, text)
}

func loop(dev *ssd1306.Dev, inet string) error {
	src := image1bit.NewVerticalLSB(dev.Bounds())
	img := convert(dev, src)

	info.hostname = getHostname()
	info.ipv4 = GetInternalIP(inet)
	
	if err := dev.Draw(dev.Bounds(), img, image.Point{}); err != nil {
		return err
	}
	return nil
}

func mainImpl() error {
	log.Printf("starting rpi-monitor service using /dev/i2c-%s", *args.i2cID)
	defer log.Printf("stopping the rpi-monitor service")

	if _, err := host.Init(); err != nil {
		return err
	}

	// Open the device on the right bus.
	var s *ssd1306.Dev
	opts := ssd1306.Opts{W: *args.width, H: *args.height, Rotated: *args.rotated, Sequential: *args.sequential, SwapTopBottom: *args.swapTopBottom}
	c, err := i2creg.Open(*args.i2cID)
	if err != nil {
		return err
	}
	defer c.Close()
	if args.hz != 0 {
		if err = c.SetSpeed(args.hz); err != nil {
			return err
		}
	}
	if p, ok := c.(i2c.Pins); ok {
		log.Printf("Using pins SCL: %s  SDA: %s", p.SCL(), p.SDA())
	}
	s, err = ssd1306.NewI2C(c, &opts)
	if err != nil {
		return err
	}

	for {
		err = loop(s, *args.inet)
		if err != nil {
			log.Printf("error: %s", err)
			return err
		}
		time.Sleep(1 * time.Second)
	}
}

func main() {
	args.i2cID = flag.String("i2c", "1", "I²C bus to use")
	args.inet = flag.String("inet", "eth0", "net interface to query for IPv4")
	flag.Var(&args.hz, "hz", "I²C bus/SPI port speed")
	args.height = flag.Int("h", 32, "display height")
	args.width = flag.Int("w", 128, "display width")
	args.rotated = flag.Bool("r", false, "rotate the display by 180°")
	args.sequential = flag.Bool("n", false, "sequential/interleaved hardware pin layout")
	args.swapTopBottom = flag.Bool("s", false, "swap top/bottom hardware pin layout")
	flag.Parse()

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		if err := mainImpl(); err != nil {
			log.Fatalf("ssd1306: %s", err)
		}
	}()

	<-done
}
# Talos Extension Service

RaspberryPi display information via ssd1306 OLED display

Based on https://github.com/periph/cmd/tree/main/ssd1306

## Usage

Enable the extension in the machine configuration before installing Talos:

```yaml
machine:
  install:
    extensions:
      - image: ghcr.io/linuxmaniac/rpi-monitor:<VERSION>
```

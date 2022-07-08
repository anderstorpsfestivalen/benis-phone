![benis-phone Logo](/logo.jpeg)

# benis-phone
Best Enterprise Network Integrated Soft-phone, aka benis-phone!

# Requirements
To run on Linux (tested with Debian / Ubuntu / Raspbian) the following packets are required: 

* pkg-config 
* libasound2-dev 
* build-essential 

Install with: apt install pkg-config libasound2-dev build-essential 

# Sound on RPI with an USB-card
If running on a RPI, install pulseaudio and disable the onboard soundcard by commenting out the following in /lib/modprobe.d/aliases.conf

```
#options snd-usb-audio index=-2
```

Also add a blacklist entry in /etc/modprobe.d/raspi-blacklist.conf

```
blacklist snd_bcm2835
```

Reboot the RPI!

# Credentials
Create a dir called "creds" in the root, then create a file called creds.json, the file should look like this:

```
{
        "S3": {
                "Key": "xxx",
                "Secret": "xxx"
        },
        "Polly": {
                "Key": "xxx",
                "Secret": "xxx"
        },
        "Backend": {
                "Username": "xxx",
                "Password": "xxx"
        },
        "Trafiklab": "xxx",
        "Systemet": "xxx",
        "MediaServer": "xxx",
        "HTTPServerAuth": {
                "Username": "xxx",
                "Password": "xxx"
        }
}
```

# Recoding
To get recording to work, create in the files a directory called "recoding".

# Running
To run as a virtual phone taking keyboard inputs, run without any arguments

To run with a real phone connected and using DTMF, start with -phone

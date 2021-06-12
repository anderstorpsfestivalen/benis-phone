![benis-phone Logo](/logo.jpeg)

# benis-phone
Best Enterprise Network Integrated Soft-phone, aka benis-phone!

# Requirements
To run on Linux (tested with Debian / Ubuntu / Raspbian) the following packets are required: 

* pkg-config 
* libasound2-dev 
* build-essential 

Install with: apt install pkg-config libasound2-dev build-essential 

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
  "Systemet": "xxx"
}
```

# Recoding
To get recording to work, create in the root a directory called "temp" and a directory called "random" in the "temp" directory.

# Running
Just start without any arguments!
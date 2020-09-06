#!/usr/bin/python
# -*- coding: utf-8 -*-

import sys
from datetime import datetime
from datetime import *
from dateutil.relativedelta import *
import dateutil.parser
import boto3
import creds


def main():
    # Prepare message
    message = time_left()
    # Send message string to polly
    polly(message)


def time_left():
    now = datetime.now()
    #closing = datetime(2020, 3, 9, 3, 0, 0, 100000)
    closing = now + relativedelta(days=+1, hour=3, minute=0, second=0)
    diff = closing - now
    d = dateutil.parser.parse(str(diff))
    print (d.hour, d.minute, d.second)

    # Prepare message
    message = ""
    message = message + "Baren st\xc3\xa4nger, om, " + str(d.hour) + \
        ", timmar, och, " + str(d.minute) + ", minuter, och, " + \
        str(d.second) + ", sekunder."
    # Debug
    print (message)

    # Return message
    return message


def polly(message):
    polly_client = boto3.Session(
        aws_access_key_id=creds.aws_key,
        aws_secret_access_key=creds.aws_secret,
        region_name='eu-north-1').client('polly')

    response = polly_client.synthesize_speech(VoiceId='Astrid',
                                              OutputFormat='mp3',
                                              Text=message)

    file = open('output.mp3', 'wb')
    file.write(response['AudioStream'].read())
    file.close()


if __name__ == '__main__':
    main()

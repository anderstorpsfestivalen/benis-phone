#!/usr/bin/python
# -*- coding: utf-8 -*-

import json
import requests
import sys
from datetime import datetime
import dateutil.parser
import boto3
import creds


def main():
    # Gather API data
    message = gather_api_data()
    # Send message string to polly
    polly(message)


def gather_api_data():
    request_station_info = '''
    <REQUEST>
      <LOGIN authenticationkey="''' + creds.trafiklab_key + '''"/>
      <QUERY objecttype="TrainStation" schemaversion="1">
            <FILTER>
                  <EQ name="Advertised" value="true" />
                  <EQ name="AdvertisedLocationName" value="Reftele" />
            </FILTER>
      </QUERY>
    </REQUEST>
    '''

    request_time_stable = '''
    <REQUEST>
        <LOGIN authenticationkey="''' + creds.trafiklab_key + '''"/>
        <QUERY objecttype="TrainAnnouncement" schemaversion="1.3" orderby="AdvertisedTimeAtLocation">
                <FILTER>
                    <AND>
                            <EQ name="ActivityType" value="Avgang" />
                            <EQ name="LocationSignature" value="Rft" />
                            <OR>
                                <AND>
                                        <GT name="AdvertisedTimeAtLocation" value="$dateadd(-00:15:00)" />
                                        <LT name="AdvertisedTimeAtLocation" value="$dateadd(14:30:00)" />
                                </AND>
                                <AND>
                                        <LT name="AdvertisedTimeAtLocation" value="$dateadd(00:30:00)" />
                                        <GT name="EstimatedTimeAtLocation" value="$dateadd(-00:15:00)" />
                                </AND>
                            </OR>
                    </AND>
                </FILTER>
        </QUERY>
    </REQUEST>
    '''

    station_info = requests.post(
        'https://api.trafikinfo.trafikverket.se/v1/data.json', data=request_station_info).json()
    # print(station_info)

    time_stable = requests.post(
        'https://api.trafikinfo.trafikverket.se/v1/data.json', data=request_time_stable).json()
    # print(time_stable)

    for x in time_stable['RESPONSE']['RESULT']:

        for y in x['TrainAnnouncement']:
            ActivityType = y['ActivityType']
            TechnicalTrainIdent = y['TechnicalTrainIdent']
            AdvertisedTimeAtLocation = y['AdvertisedTimeAtLocation']
            InformationOwner = y['InformationOwner']
            ProductInformation = y['ProductInformation'][0]
            TrackAtLocation = y['TrackAtLocation']
            TypeOfTraffic = y['TypeOfTraffic']

        for z in y['FromLocation']:
            StationName = z['LocationName']
            FromLocation = convert_StationName(StationName)

        for z in y['ToLocation']:
            StationName = z['LocationName']
            ToLocation = convert_StationName(StationName)

        # for z in y['ViaToLocation']:
        #    StationName = z['LocationName']
        #    ViaToLocation = convert_StationName(StationName)

        # Fix time
        d = dateutil.parser.parse(AdvertisedTimeAtLocation)

        # Manipulate track number if 1 (pronounced "en" otherwise)
        if '1' in TrackAtLocation:
            FixedTrackAtLocation = "ett"
        else:
            FixedTrackAtLocation = TrackAtLocation

        # Prepare message
        message = ""
        message = message + InformationOwner + ", " + \
            ProductInformation + ", " + TypeOfTraffic + " nummer, " + TechnicalTrainIdent + ", " \
            + unicode("Fr\xc3\xa5n, ", "UTF-8") + FromLocation + ", " + "Till, " + ToLocation \
            + ", " + unicode("avg\xc3\xa5r fr\xc3\xa5n sp\xc3\xa5r, ", "UTF-8") \
            + FixedTrackAtLocation + ", klockan, " \
            + str(d.hour) + ", och, " + str(d.minute)

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


def convert_StationName(StationName):
    request_short_name_to_full_name = '''
    <REQUEST>
      <LOGIN authenticationkey="''' + creds.trafiklab_key + '''"/>
      <QUERY objecttype="TrainStation" schemaversion="1">
            <FILTER>
                  <EQ name="Advertised" value="true" />
                  <EQ name="LocationSignature" value="''' + StationName.encode('UTF-8') + '''" />
            </FILTER>
            <INCLUDE>AdvertisedLocationName</INCLUDE>
      </QUERY>
    </REQUEST>
    '''
    short_name_to_full_name = requests.post(
        'https://api.trafikinfo.trafikverket.se/v1/data.json', data=request_short_name_to_full_name).json()

    for x in short_name_to_full_name['RESPONSE']['RESULT']:
        for y in x['TrainStation']:
            return y['AdvertisedLocationName']


if __name__ == '__main__':
    main()

#!/usr/bin/python

import subprocess
import os
import random
import time
import RPi.GPIO as GPIO

telefon_lyft = False
played_once = False


def init():
    GPIO.setmode(GPIO.BCM)
    GPIO.setup(6, GPIO.IN, pull_up_down=GPIO.PUD_UP)


def play():
    global telefon_lyft
    global played_once
    file_name = random.choice(os.listdir(
        "/home/pi/mp3-test/wav/kaj-och-borje"))
    # Wait a short time before playing
    time.sleep(0.75)
    subprocess.call(
        ['aplay /home/pi/mp3-test/wav/kaj-och-borje/%s' % file_name], shell=True)
    telefon_lyft = False
    played_once = True


def main():
    init()
    global telefon_lyft
    global played_once
    while True:
        input_state = GPIO.input(6)
        if input_state == False:
            print("DEBUG: Luren ar palagd")
            time.sleep(0.2)
            played_once = False
        if input_state == True:
            print("DEBUG: Luren ar lyft")
            if telefon_lyft == False and played_once == False:
                telefon_lyft = True
                play()
            time.sleep(0.2)


if __name__ == '__main__':
    main()

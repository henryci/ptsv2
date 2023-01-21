# ptsv2
Post Test Server v2

## Overview
Post Test Server V2 (PTSV2) was the evolution of a testing tool I created in the early days of Localytics to facilitate testing of our mobile clients. I put the first version, an extremely simple PHP script, online at posttestserver.com and to [Dreamhost](https://www.dreamhost.com/)'s credit, they put up with the unsurprising amount of spam a server that accepts input from anybody on the Internet receives. But over time, the feature requests and spam issues prompted me to create a new version, which is what we have here.

**PTSV2** Is a Golang service designed to be run on [AppEngine](https://cloud.google.com/appengine). It receives requests (GET, and POST, despite what the name may say) and stores them as objects in [Datastore](https://cloud.google.com/datastore). These objects can be retrieved in a variety of formats for human or programatic consumption.

## Why did this exist?
I found the original version helpful when I was debugging mobile client SDK code. From all the positive emails I received (And I can't thank you all enough!) it seems like a lot of other people have been testing uploads and data transmission in all sorts of weird environments.

## Why doesn't it exist anymore?
I ran this service for almost 15 years. I originally did it because it was very little effort so if it helped anybody then that was neat. Then I redid it in Golang as an excuse to learn the language and that was fun. However, the Internet is a dodgy place and the years of spam and abuse eventually wore me down. The final straw was a very well orchestrated DDOS that managed to rack up such a GCP bill that I had to press the panic button and take it all down. I don't have the time or energy to figure out how to investigate and fix the exploit so, as [Paula Poundstone](https://en.wikipedia.org/wiki/Paula_Poundstone) once said: this is why we can't have nice things.

## Can you run it?
Absolutely. Bear in mind that if you expose it to the internet all sorts of weird bots WILL find it and WILL spam it, either intentionally or not so do so at your own risk. But for internal testing - it's great. There are two options for running it:

1. Run it locally using [Google Cloud Local Server](https://cloud.google.com/appengine/docs/legacy/standard/python/tools/using-local-server). Originally I wanted to put instructions here on how to do that but it has changed since the last time I touched this (which was some years ago) so the best I can do is leave you with that link. Sorry. :(
2. Deploy it to Appengine. Again, the methodolgy for this has changed since I've done it last, but this is probably pretty quick.

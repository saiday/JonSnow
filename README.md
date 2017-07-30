# Jon Snow the watcher
> Jon Snow is elected the 998th Lord Commander of the Night's Watch.

![](doc/screenshot.png)

Jon Snow made App Store, Google Play review monitoring easy, build your own service by one click, few configs.  
No codes needed.

## Deploy your own app to heroku (shortcuts)
[![Deploy](https://www.herokucdn.com/deploy/button.svg)](https://heroku.com/deploy?template=https://github.com/saiday/JonSnow&env[JON_SNOW_GOOGLE_PLAY_LOCATION]=zh-tw&env[JON_SNOW_APP_STORE_LOCATION]=tw) (targeting tw store)

[![Deploy](https://www.herokucdn.com/deploy/button.svg)](https://heroku.com/deploy?template=https://github.com/saiday/JonSnow&env[JON_SNOW_GOOGLE_PLAY_LOCATION]=en&env[JON_SNOW_APP_STORE_LOCATION]=us) (targeting us store)


### One more thing (Sync new reviews)

You have to add cron job on your heroku service by yourself.  
The executable binary located at `bin/JonSnow` which is your scheduling target.

You can follow our simple instruction: [Add cron job on heroku](https://github.com/saiday/JonSnow/wiki/Add-cron-job-on-heroku) as well.

## Contact
[@saiday](https://twitter.com/saiday)


This project is inspired by [LaunchKit](https://launchkit.io/) and [go-google-play-review-watcher](https://github.com/Konboi/go-google-play-review-watcher)

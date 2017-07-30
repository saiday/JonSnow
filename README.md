# Jon Snow the watcher
> Jon Snow is elected the 998th Lord Commander of the Night's Watch.

![](doc/screenshot.png)

I made it easy for you, build your own monitoring service by one click, few configs. No codes needed.

Deploy your own app to heroku:  
[![Deploy](https://www.herokucdn.com/deploy/button.svg)](https://heroku.com/deploy?template=https://github.com/saiday/JonSnow&env[JON_SNOW_GOOGLE_PLAY_LOCATION]=zh-tw&env[JON_SNOW_APP_STORE_LOCATION]=tw) (targeting tw store)

[![Deploy](https://www.herokucdn.com/deploy/button.svg)](https://heroku.com/deploy?template=https://github.com/saiday/JonSnow&env[JON_SNOW_GOOGLE_PLAY_LOCATION]=en&env[JON_SNOW_APP_STORE_LOCATION]=us) (targeting us store)


#### One more thing

Congratulations, you've got slack message after your heroku app deployed.  
One more thing to do, add cron job on your service.

Hit Manage App button  
![](doc/deployed.png)

Hit Heroku Scheduler in Add-ons section  
![](doc/heroku-scheduler.png)

You will see this page, hit Add new job button  
![](doc/add-job.png)

Fill our command: `$ bin/JonSnow`, select preferred frequency and save.  
![](doc/job.png)

DONE.

## Contact
[@saiday](https://twitter.com/saiday)


This project is inspired by [LaunchKit](https://launchkit.io/) and [go-google-play-review-watcher](https://github.com/Konboi/go-google-play-review-watcher)

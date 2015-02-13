[![Circle CI](https://circleci.com/gh/goura/spotmc.svg?style=svg)](https://circleci.com/gh/goura/spotmc)

SpotMC
=======
SpotMC is a utility to host a craftbukkit/minecraft_server type games on an AWS EC2 spot instance.

Using SpotMC, you might be able to:
* Start your game server on a spot instance, which is very cheap in price
* Save your game state when terminating a spot instance for future play
* Automatically terminate your spot instance after a certain idle time so that you won't get billed a lot just because you forgot to terminate it
* Set a hard-limit of an uptime so that you don't have to worry about unexpected bills

It is aimed to let you host a craftbukkit/minecraft server to play with your family, friends.

Status
=======
This project is still in its early development process and nothing is done yet.

I've been running a minecraft server on a spot instance for my family
for a year using adhoc shell scripts to autoscale and auto-terminate,
and I'm now trying to port it into a more solid something (to learn Go).

What You Have to Prepare
=========================
- Access to AWS Management Console. You are going to setup an AutoScaling group. Game state will be saved in S3.
- Game server jar file (like craftbukkit.jar/minecraft_server.jar) and its eula.txt (for minecraft). It's not included in this software.

Parameters
===========

spotmc command uses env vars for configuration

* `SPOTMC_SERVER_JAR_URL` (mandatory)
    * Specify the URL of the game server jar in `s3://{bucket}/{key}` format

* `SPOTMC_SERVER_EULA_URL` (mandatory)
    * Specify the URL of the eula.txt file in `s3://{bucket}/{key}` format

* `SPOTMC_DATA_URL` (mandatory)
    * Specify the path where you like to save the data in `s3://{bucket}/{key}` format. Currently spotmc saves the data as a single tgz file.

* `SPOTMC_JAVA_PATH` (mandatory)
    * Specify the full path to java cmd (like `/usr/bin/java`).

* `SPOTMC_JAVA_ARGS` (default=none)
    * Extra args to give to java cmd, like `-Xmx1024M -Xms1024M`

* `SPOTMC_MAX_IDLE_TIME` (default=14400)
    * The time which after everyone logs out from the server, spotmc tries to terminate the instance. Specify this in seconds.

* `SPOTMC_MAX_UPTIME` (default=43200)
    * The time after which no matter whether someone is still playing or not, the server will terminate. Specify this in seconds.

* `SPOTMC_IDLE_WATCH_PATH` (default="world/playerdata")
    * The directory, relative to game data root, to watch for the game activity. If the specified path is inactive (i.e. doesn't get updated) for `SPOTMC_MAX_IDLE_TIME`, spotmc tries to shutdown the cluster specified in `SPOTMC_AUTOSCALING_GROUP`.


* `SPOTMC_DDNS_UPDATE_URL` (default=none)
    * When spotmc starts, it accesses this URL. Use it to update your DDNS settings.
    * If this parameter is not specified, spotmc won't do anything.

* `SPOTMC_AUTOSCALING_GROUP` (default=none)
    * When spotmc decides to shutdown the whole cluster, it tries to shutdown the autoscaling group. Specify the name of the group.
    * If this paramter is not specified, spotmc won't touch the autoscaling group.

* `SPOTMC_AWS_REGION` (default="ap-northeast-1")
    * Which AWS region to use

* `SPOTMC_KILL_INSTANCE_MODE` (default="false")
    * spotmc tries to kill the instance when the game server goes down for some reason, or when it detected the spot instance termination notification
    * When this parameter is set to "false" it will not actually shutdown the instance. This is just for safety not to casually kill your server.
    * When this parameter is set to "shutdown" it will call `SPOTMC_SHUTDOWN_CMD` to kill the instance. This is a recommended parameter.

* `SPOTMC_SHUTDOWN_CMD` (default="/sbin/shutdown -h now")
    * The command called when spotmc is killing the instance

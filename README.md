SpotMC
=======
SpotMC is a utility to host a craftbukkit/minecraft-server type games on an AWS EC2 spot instance.

Using SpotMC, you might be able to:
* Start your game server on spot instances, which is very cheap in price
* Save your game state when terminating a spot instance for future play
* Automatically terminate your spot instance after a certain idle time so that you won't get billed a lot just because you forgot to terminate it
* Set a hard-limit of an uptime so that you don't have to worry about unexpected bills

Status
=======
This project is still in its early development process and nothing is done yet.

I've been running a minecraft server on a spot instance for my family
for an year using adhoc shell scripts to autoscale and auto-terminate,
and I'm now trying to port it into a more solid something (to learn Go).

What You Have to Prepare
=========================
- Access to AWS Management Console. You are going to setup a AutoScaling group. Game state will be saved in S3.
- Game server jar file (like craftbukkit.jar/minecraft-server.jar) and it's eula.txt (for minecraft). It's not included in this software.

Parameters
===========
TBD

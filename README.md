# How to launch client 
To launch the client, simply run command `go run client.go` in the folder with client implementation from this repo. Basically, you can see all instructions in the terminal during communication with the client. 
* When you start the client you should write in the terminal server's ip address and port (default port is 5454)
* Then you can choose whether you want create a new room or enter the id of an existing room
* After that you enter your name in this room
* At this point, you can communicate with the server. 
  * If you wrtie command `\SendVoice FileName` then you'll send a file with the name **FileName** with your voice message to all clients in the same room as you
  * If you write command `\GetUsers` then you'll see in terminal names of all users from your room
  * If you write command `\Stop` then you'll stop your current client and also leave the current room

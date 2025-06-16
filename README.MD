# GoHttpRequestTaskProxy

GoHttpRequestTaskProxy is a service written in Go that redirects requests from the user to a third party service sent to the server through a post request to the /TASK/ endpoint written in a json format.

<b>This service is made purely for educational purposes, is in Work In Progress and isn't ready for production. </b>

The service uses a postgres database to store the tasks.

# Routing

POST   /task/              :  create a task, returns ID

GET    /task/<taskid>      :  returns a single task by ID

GET    /task/              :  returns all tasks

DELETE /task/<taskid>      :  delete a task by ID

DELETE /task/			   :  delete all tasks

# How to run

If you want to run this out of the deployment folder:

    docker compose up -d go_db
    docker compose up -d go-app

If you  want to run the app out of the root folder:

    docker compose -f deployments/docker-compose.yml up -d go-app
t
## Running the server locally

set enviroment variable "DATABASE_URL" to

    host = go_db user=postgres password=postgres dbname=postgres sslmode=disable 
After that, it is possible run it locally with 

    go run main.go

You can use the win_runLocally.ps1 script in scripts/ to  do that for you on Windows.


# Roadmap

- [X] Implement basic prototype
- [X] Add docker container
- [ ] Implement the database
- [ ] Port code to work with the database instead of an array
- [ ] Add unit tests
- [ ] Add get request parameterization 

# GoHttpRequestTaskProxy

GoHttpRequestTaskProxy is a web service written in Go that redirects requests from the user to a third party service sent to the server through a post request to the /TASK/ endpoint written in a json format.

<b>This service is made purely for educational purposes, is work in progress and isn't ready for production. </b>

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

If you  want to run the app out of the project root folder:

    docker compose -f deployments/docker-compose.yml up -d go-app

## Running the server locally

set enviroment variable "DATABASE_URL" to

    host = go_db user=postgres password=postgres dbname=postgres sslmode=disable 
After that, it is possible run it locally with 

    go run main.go

You can use the win_runLocally.ps1 script in main folder to do that for you on Windows.


# Roadmap

- [X] Implement basic prototype
- [X] Add docker containers (postgres database server, REST server)
- [X] Implement the database
- [X] Port code to work with the database instead of an array
- [ ] Implement http authorization
- [ ] Add unit tests
- [ ] Create OpenAPI schema
- [ ] Add get request parameterization 

# Issues

- Body {} in request can't be nil, add * before string?
- Database repository isn't separate and is inside the controller. (Since there is only one controller using the database this is fine now, but if I want to use the same database in other controllers in the future there will be duplicate code everywhere)
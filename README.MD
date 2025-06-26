# GoHttpRequestTaskProxy

GoHttpRequestTaskProxy is a web service written in Go that redirects requests from the user to a third party service sent to the server through a post request to the /TASK/ endpoint written in a json format.

<b>This service is made purely for educational purposes, is work in progress and isn't ready for production. </b>

The service uses a postgres database to store the tasks.

# Routing

POST   /task/              :  create a task, returns ID

GET    /task/<taskid>      :  returns a single task by ID

GET    /task/              :  returns all tasks. Can be filtered by status, httpStatusCode, minScheduledStartTime, maxScheduledStartTime, minScheduledEndTime, maxScheduledEndTime

DELETE /task/<taskid>      :  delete a task by ID

DELETE /task/			   :  delete all tasks. Can be filtered by status, httpStatusCode, minScheduledStartTime, maxScheduledStartTime, minScheduledEndTime, maxScheduledEndTime

# How to run

If you want to run this out of the deployment folder:

    docker compose up -d

If you want to run the app out of the project root folder:

    docker compose -f deployments/docker-compose.yml up -d

Keep in mind that the application container has to be rebuilt everytime the code base changes.

The docker compose deployment consists of:
- go_app: the server application
- go_db: the postgres database

# Running tests

<b>Make sure to run the tests inside the test docker container and to set the enviroment variables to the test container.
Running the tests inside the production container WILL result in test failures and data loss. </b>

The test postgres docker container can be ran out of the deployments folder like this:
    docker compose up -d

And out of the project root folder like this:
    docker compose -f deployments/docker-compose.test.yml up --build

The test docker deployment consists of service:
- go_testapp: the test server application
- go_testdb: the postgres test database

## Running tests locally
set enviroment variable "DATABASE_URL" to

    postgresql://testpostgres:testpostgres@localhost:2345
Launch postgres test server with the correct credentials.
After that, it is possible run it locally with 

    go test ./cmd/go-http-request-task-proxy
You can also use the scripts/win_runTests.ps1 out of the project root folder to set the enviroment variable before running the test.

# Roadmap

- [X] Implement basic prototype
- [X] Add docker containers (postgres database server, REST server)
- [X] Implement the database
- [X] Port code to work with the database instead of an array
- [X] Implement basic http authentication
- [X] Add unit tests
- [X] Create OpenAPI schema
- [X] Add basic get request parameterization 
- [X] Add basic get request unit tests
- [X] Add basic delete request parametrization & unit tests
- [X] Implement workers
- [X] Fix unit tests
- [X] Add scheduled_start_time and scheduled_end_time parametrization (by min and max)
- [X] Add unit tests for scheduled_start_time and scheduled_end_time parametrization

# Issues / TODO

- In the tasks, the body is displayed incorrectly on get request, with the escape characters displayed as their escape sequence (\n, \). This is related to ginContext.JSON(). Relevant stackoverflow issue: https://stackoverflow.com/questions/73460507/json-response-in-gogin-without-escape-characters (actually this was probably an issue with the server i was using to test this with)
- can't query by content length, headers (in url json format?)
- can't query by times directly by using the equals symbol("="). The database stores times in 2025-06-23 19:14:23.052463 format (YYYY-MM-DD HH:MM:SS.mmmmmm) and golang's default formats only include nanosecond formats. I would probably need to write a format for this myself, preferably one that is inline with the database's format to implement searching for tasks with those times.  


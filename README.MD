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

If you want to run the app out of the project root folder:

    docker compose -f deployments/docker-compose.yml up -d go-app
    docker compose -f deployments/docker-compose.yml up -d go_db

## Running the server locally

set enviroment variable "DATABASE_URL" to

    postgresql://postgres:postgres@localhost:5432
After that, it is possible run it locally with 

    go run main.go

You can use the win_runLocally.ps1 script in main folder to do that for you on Windows.

## Running tests

<b>Make sure to run the tests inside the test docker container and to set the enviroment variables to the test container.
Running the tests inside the production container WILL result in test failures and data loss. </b>

The test docker container can be ran out of the deployments folder like this:
    docker compose up -d go_testdb

And out of the project root folder like this:
    docker compose -f deployments/docker-compose.yml up -d go_testdb

set enviroment variable "DATABASE_URL" to

    postgresql://testpostgres:testpostgres@localhost:5432
After that, it is possible run it locally with 

    go test
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
- [ ] Add delete request parametrization

# Issues

- In the tasks, the body is displayed incorrectly on get request, with the escape characters displayed as their escape sequence (\n, \). This is related to ginContext.JSON(). Relevant stackoverflow issue: https://stackoverflow.com/questions/73460507/json-response-in-gogin-without-escape-characters (actually this was probably an issue with the server i was using to test this with)
- There are no unit tests for: if the scheduledStartTime and the scheduledEndTime are correct, if the in-progress task even exists (if the task is not done).
- Can't run test database docker and server database docker at the same time (they use the same ports)
- The ability to query the get /task endpoint with startDate greater or less than is unimplemented. Its implemention is made difficult as gin only supports "=" parameters. Technically it would be possible to implement "=<" "=>" operators by parsing them quite easily. Although "<" and ">" operators would requiring parsing the whole request url (given that the extra characters don't fail gin's parser, that would mean I would have to make my own parser for all parameters).
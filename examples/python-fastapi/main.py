from fastapi import FastAPI, Request

app = FastAPI()


@app.get("/")
def root(request: Request) -> dict:
    return {"served_from": "a boxer sandbox", "host": request.headers.get("host")}

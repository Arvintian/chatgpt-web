from nuxt import route, logger, Request
from nuxt.repositorys.validation import fields, use_args
import traceback
import tiktoken

encoding_cache = {}

support_models = set(["gpt-3.5-turbo",
                      "gpt-3.5-turbo-16k",
                      "gpt-4",
                      "gpt-4-32k",
                      "gpt-4o",
                      "gpt-4o-mini"])


@route("/tokenizer/<str:model_name>", methods=["POST"])
@use_args({
    "role": fields.Str(required=True),
    "content": fields.Str(required=True),
    "name": fields.Str(required=False)
}, location="json")
def get_num_tokens(req: Request, message: dict, model_name: str):
    try:
        return {
            "code": 200,
            "num_tokens": num_tokens_from_messages([message], model=model_name)
        }
    except Exception as e:
        logger.error(traceback.format_exc())
        return {
            "code": 500,
            "msg": "{}".format(e)
        }


def num_tokens_from_messages(messages, model="gpt-3.5-turbo"):
    """Returns the number of tokens used by a list of messages."""
    encoding = None
    if model in encoding_cache:
        encoding = encoding_cache.get(model)
    else:
        try:
            encoding = tiktoken.encoding_for_model(model)
        except KeyError:
            encoding = tiktoken.get_encoding("cl100k_base")
        encoding_cache[model] = encoding
    if model in support_models:  # note: future models may deviate from this
        return _inner_num_tokens_from_messages(messages, encoding, model)
    else:
        raise NotImplementedError(f"""num_tokens_from_messages() is not presently implemented for model {model}.
See https://cookbook.openai.com/examples/how_to_count_tokens_with_tiktoken for information on how messages are converted to tokens.""")


def _inner_num_tokens_from_messages(messages, encoding, model):
    """Return the number of tokens used by a list of messages."""
    if model in {
        "gpt-3.5-turbo-0613",
        "gpt-3.5-turbo-16k-0613",
        "gpt-4-0314",
        "gpt-4-32k-0314",
        "gpt-4-0613",
        "gpt-4-32k-0613",
    }:
        tokens_per_message = 3
        tokens_per_name = 1
    elif model == "gpt-3.5-turbo-0301":
        tokens_per_message = 4  # every message follows <|start|>{role/name}\n{content}<|end|>\n
        tokens_per_name = -1  # if there's a name, the role is omitted
    elif "gpt-3.5-turbo" in model:
        return _inner_num_tokens_from_messages(messages, encoding, model="gpt-3.5-turbo-0613")
    elif "gpt-4" in model:
        return _inner_num_tokens_from_messages(messages, encoding, model="gpt-4-0613")
    else:
        raise NotImplementedError(f"""num_tokens_from_messages() is not implemented for model {model}.""")
    num_tokens = 0
    for message in messages:
        num_tokens += tokens_per_message
        for key, value in message.items():
            num_tokens += len(encoding.encode(value))
            if key == "name":
                num_tokens += tokens_per_name
    num_tokens += 3  # every reply is primed with <|start|>assistant<|message|>
    return num_tokens

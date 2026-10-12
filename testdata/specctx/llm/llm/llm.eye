# A model gateway: text in, text out.

contract Llm {
  query complete(prompt: String) -> String
}

var isPalindrome = function (x) {
  if (x < 0) return false;

  var reverse = 0;
  var c = x;
  while (c) {
    reverse = reverse * 10 + (c % 10);
    c = Math.floor(c / 10);
  }

  return reverse == x;
};

console.log(isPalindrome(898)); // true

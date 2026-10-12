# SPECS.md section 10, example 6.
test "NaN equals itself" {
  let zero = 0.0
  let nan = zero / zero
  expect(nan == nan && [nan].contains(nan) && !(zero == -zero))
  expect("{nan} {1.0 / zero} {-1.0 / zero}" == "NaN +Inf -Inf")
}

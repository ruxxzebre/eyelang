type Shape =
  | Circle(radius: Float)
  | Rect(width: Float, height: Float)
  | Polygon(points: List((Float, Float)))

trait Area {
  fn area(self) -> Float
  fn describe(self) -> String = "an area of {self.area()}"
}

impl Area for Shape {
  fn area(self) -> Float = match self {
    Circle(radius) => 3.14159 * radius * radius
    Rect(width, height) => width * height
    Polygon(points) => shoelace(points)
  }
}

fn shoelace(points: List((Float, Float))) -> Float {
  var sum = 0.0
  for (i, (x1, y1)) in points |> enumerate {
    let (x2, y2) = points[(i + 1) % size(points)] ?? (0.0, 0.0)
    sum += x1 * y2 - x2 * y1
  }
  abs(sum) / 2
}

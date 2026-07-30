radio.onReceivedNumber(function (receivedNumber) {
    basic.clearScreen()
    basic.showString("" + (receivedNumber))
})
input.onButtonPressed(Button.A, function () {
    radio.sendValue("SOCKET1", 0)
    basic.showIcon(IconNames.No)
})
radio.onReceivedString(function (receivedString) {
    basic.clearScreen()
    basic.showString(receivedString)
})
input.onButtonPressed(Button.B, function () {
    radio.sendValue("SOCKET1", 1)
    basic.showIcon(IconNames.Yes)
})
radio.onReceivedValue(function (name, value) {
    basic.clearScreen()
    basic.showString("" + name + "=" + value)
})
basic.showIcon(IconNames.SmallHeart)
radio.setGroup(1)
radio.setTransmitSerialNumber(true)
basic.showIcon(IconNames.Heart)

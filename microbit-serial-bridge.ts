radio.onReceivedNumber(function (receivedNumber) {
    sendPacket("N", "" + receivedNumber)
})
function toHex (n: number) {
    for (let shift = 28; shift >= 0; shift -= 4) {
        s += HEX_CHARS.charAt((n >>> shift) & 0xF)
    }
return s
}
// Indikace příjmu (rádio -> sériová linka) na horních dvou řádcích displeje (y = 0..1).
function indicateReceive () {
    led.toggle(cntRx % 5, cntRx / 5)
    cntRx += 1
    if (cntRx > 9) {
        cntRx = 0
    }
}
// Indikace odesílání (sériová linka -> rádio) na dalších dvou řádcích displeje (y = 2..3).
function indicateSend () {
    led.toggle(cntTx % 5, 2 + cntTx / 5)
    cntTx += 1
    if (cntTx > 9) {
        cntTx = 0
    }
}
radio.onReceivedString(function (receivedString) {
    sendPacket("S", receivedString)
})
// Společné skládání a odeslání zprávy pro všechny typy packetů.
// type: jednopísmenný identifikátor ("N" | "S" | "V"), payload: už naformátovaný obsah.
function sendPacket (_type: string, payload: string) {
    srn = radio.receivedPacket(RadioPacketProperty.SerialNumber)
    sig = radio.receivedPacket(RadioPacketProperty.SignalStrength)
    tmr = radio.receivedPacket(RadioPacketProperty.Time)
    serial.writeLine("|" + _type + "|" + toHex(srn) + "|" + sig + "|" + tmr + "|" + payload + "|")
    indicateReceive()
}
radio.onReceivedValue(function (name, value) {
    sendPacket("V", "" + name + "=" + value)
})
// Přijme ze sériové linky řádek ve formátu "|typ|hodnota|" (typ je N, S nebo V)
// a odešle odpovídající packet radiem. U typu V se hodnota posílá jako "jméno=číslo".
serial.onDataReceived(serial.delimiters(Delimiters.NewLine), function () {
    line = serial.readUntil(serial.delimiters(Delimiters.NewLine))
    parts = line.split("|")
    if (parts.length < 3) {
        return
    }
    indicateSend()
    _type = parts[1]
    value = parts[2]
    if (_type == "N") {
        radio.sendNumber(parseFloat(value))
    } else if (_type == "S") {
        radio.sendString(value)
    } else if (_type == "V") {
        kv = value.split("=")
        if (kv.length == 2) {
            radio.sendValue(kv[0], parseFloat(kv[1]))
        }
    }
})
let kv: string[] = []
let value = ""
let _type = ""
let parts: string[] = []
let line = ""
let tmr = 0
let sig = 0
let srn = 0
let cntTx = 0
let cntRx = 0
let s = ""
let HEX_CHARS = "0123456789ABCDEF"
serial.redirect(
SerialPin.USB_TX,
SerialPin.USB_RX,
BaudRate.BaudRate115200
)
radio.setTransmitPower(7)
radio.setGroup(1)
basic.forever(function () {
    pause(1000)
led.toggle(4, 4)
})

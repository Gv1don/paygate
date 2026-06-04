import { Button, Input, Text, YStack, Spinner } from "tamagui"
import { useState, useEffect } from "react"

const API = "/api/v1"
const MERCHANT_REDIRECT = "https://example.com/order/"

type Step = "init" | "3ds" | "waiting" | "done" | "error"

function App() {
  const [orderID, setOrderID] = useState("")
  const [amount, setAmount] = useState("")
  const [currency, setCurrency] = useState("RUB")
  const [step, setStep] = useState<Step>("init")
  const [sessionID, setSessionID] = useState("")
  const [status, setStatus] = useState("")
  const [error, setError] = useState("")
  const [threeDSURL, setThreeDSURL] = useState("")

  const initPayment = async () => {
    setError("")
    setStatus("")
    try {
      const res = await fetch(`${API}/payments/init`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          order_id: orderID,
          user_id: "user-anonymous",
          amount: Number(amount),
          currency,
        }),
      })
      const data = await res.json()
      if (!res.ok) {
        setError(data.error || "init failed")
        return
      }
      setSessionID(data.bank_session_id)
      setStatus(data.status)
      setThreeDSURL(data.three_ds_url || "")
      setStep("3ds")
    } catch (e) {
      setError(String(e))
    }
  }

  const simulate3DS = async () => {
    setStep("waiting")
    const token = "simulated_3ds_token"
    try {
      const res = await fetch(`${API}/payments/confirm`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ bank_session_id: sessionID, token }),
      })
      const data = await res.json()
      if (!res.ok) {
        setError(data.error || "3ds failed")
        setStep("error")
        return
      }
      setStatus(data.status)
      pollStatus()
    } catch (e) {
      setError(String(e))
      setStep("error")
    }
  }

  const pollStatus = async () => {
    const maxAttempts = 20
    for (let i = 0; i < maxAttempts; i++) {
      await new Promise(r => setTimeout(r, 1500))
      try {
        const res = await fetch(`${API}/payments/status?bank_session_id=${sessionID}`)
        const data = await res.json()
        if (!res.ok) continue
        setStatus(data.status)
        if (data.status === "COMPLETED" || data.status === "FAILED") {
          setStep("done")
          return
        }
      } catch {
        // ignore
      }
    }
    setError("timeout waiting for payment result")
    setStep("error")
  }

  const reset = () => {
    setStep("init")
    setSessionID("")
    setStatus("")
    setError("")
    setThreeDSURL("")
  }

  return (
    <YStack padding="$6" space="$5" maxWidth={420} margin="0 auto">
      <Text fontSize="$10" fontWeight="bold">Paygate</Text>

      {step === "init" && (
        <YStack space="$3">
          <Text fontSize="$5">Payment details</Text>
          <Input placeholder="Order ID" value={orderID} onChangeText={setOrderID} />
          <Input
            placeholder="Amount"
            value={amount}
            onChangeText={setAmount}
            keyboardType="numeric"
          />
          <Button onPress={initPayment} disabled={!orderID || !amount}>
            Proceed to payment
          </Button>
        </YStack>
      )}

      {step === "3ds" && (
        <YStack space="$4">
          <Text fontSize="$5">Payment initiated</Text>
          <Text>Order: <strong>{orderID}</strong></Text>
          <Text>Amount: <strong>{amount} {currency}</strong></Text>
          <Text>Status: {status}</Text>
          <YStack
            backgroundColor="#f5f5f5"
            padding="$4"
            borderRadius="$4"
            borderWidth={1}
            borderColor="#ddd"
            minHeight={200}
            alignItems="center"
            justifyContent="center"
          >
            <Text fontSize="$6" marginBottom="$3">3D Secure</Text>
            <Text fontSize="$3" color="gray" marginBottom="$4">
              Bank authentication required
            </Text>
            <Button onPress={simulate3DS} backgroundColor="#000" color="#fff">
              Simulate 3DS Auth
            </Button>
            <Text fontSize="$2" color="gray" marginTop="$3">
              (in production this is an iframe from the bank)
            </Text>
          </YStack>
        </YStack>
      )}

      {step === "waiting" && (
        <YStack space="$3" alignItems="center">
          <Spinner size="large" />
          <Text>Processing payment...</Text>
          <Text fontSize="$3" color="gray">Status: {status}</Text>
        </YStack>
      )}

      {step === "done" && (
        <YStack space="$3">
          <Text
            fontSize="$8"
            color={status === "COMPLETED" ? "green" : "red"}
          >{status === "COMPLETED" ? "Paid" : "Failed"}</Text>
          <Text>Order: {orderID}</Text>
          <Text>Amount: {amount} {currency}</Text>
          <Text>Bank Session: {sessionID}</Text>
          <Button onPress={reset}>New payment</Button>
          {status === "COMPLETED" && (
            <Text
              color="blue"
              cursor="pointer"
              onPress={() => window.location.href = `${MERCHANT_REDIRECT}${orderID}`}
            >Return to merchant</Text>
          )}
        </YStack>
      )}

      {step === "error" && (
        <YStack space="$3">
          <Text fontSize="$6" color="red">Error</Text>
          <Text>{error}</Text>
          <Button onPress={reset}>Try again</Button>
        </YStack>
      )}
    </YStack>
  )
}

export default App

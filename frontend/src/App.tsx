import {
  Button,
  Text,
  YStack,
  XStack,
  Spinner,
  Card,
  Separator,
} from "tamagui"
import { useState, useEffect, useRef, useCallback } from "react"

const API = "/api/v1"

type Step = "loading" | "3ds" | "waiting" | "done" | "error"

type PaymentDetails = {
  order_id: string
  amount: number
  currency: string
  return_url: string
  three_ds_server_trans_id: string
}

function formatAmount(amount: number, currency: string): string {
  const symbols: Record<string, string> = { RUB: "₽", USD: "$", EUR: "€" }
  return `${(amount / 100).toFixed(2)} ${symbols[currency] || currency}`
}

function App() {
  const [details, setDetails] = useState<PaymentDetails | null>(null)
  const [step, setStep] = useState<Step>("loading")
  const [sessionID, setSessionID] = useState("")
  const [status, setStatus] = useState("")
  const [error, setError] = useState("")
  const pollingRef = useRef(false)

  useEffect(() => {
    const session = new URLSearchParams(window.location.search).get("session")
    if (!session) {
      setError("Invalid payment link: missing session token")
      setStep("error")
      return
    }
    setSessionID(session)
    void loadPayment(session)
  }, [])

  const loadPayment = async (session: string) => {
    try {
      const res = await fetch(`${API}/payments/status?bank_session_id=${encodeURIComponent(session)}`)
      const data = await res.json()
      if (!res.ok) {
        setError(data.error || "Payment not found")
        setStep("error")
        return
      }
      setDetails({
        order_id: data.order_id,
        amount: data.amount,
        currency: data.currency,
        return_url: data.return_url || "",
        three_ds_server_trans_id: data.three_ds_server_trans_id || "",
      })
      setStatus(data.status)
      if (data.status === "INITIATED" || data.status === "PENDING_3DS") {
        setStep("3ds")
      } else if (data.status === "COMPLETED" || data.status === "FAILED" || data.status === "REFUNDED") {
        setStep("done")
      } else {
        setStep("waiting")
        startPolling()
      }
    } catch (e) {
      setError(`Network error: ${e instanceof Error ? e.message : String(e)}`)
      setStep("error")
    }
  }

  const buildCRes = (transID: string): string => {
    const header = btoa(JSON.stringify({ alg: "none" }))
      .replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "")
    const payload = btoa(JSON.stringify({
      threeDSServerTransID: transID,
      transStatus: "Y",
    })).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "")
    return `${header}.${payload}.`
  }

  const simulate3DS = async () => {
    setStep("waiting")
    try {
      // 1. Simulate 3DS authentication → PENDING_3DS
      const cres = details ? buildCRes(details.three_ds_server_trans_id) : "simulated_cres"
      const cresRes = await fetch(`${API}/payments/3ds-return`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ bank_session_id: sessionID, cres }),
      })
      if (!cresRes.ok) {
        const data = await cresRes.json()
        setError(data.error || "3DS authentication failed")
        setStep("error")
        return
      }

      // 2. Confirm payment with token → CONFIRMED
      const token = "simulated_3ds_token"
      const confirmRes = await fetch(`${API}/payments/confirm`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ bank_session_id: sessionID, token }),
      })
      const confirmData = await confirmRes.json()
      if (!confirmRes.ok) {
        setError(confirmData.error || "Payment confirmation failed")
        setStep("error")
        return
      }
      setStatus(confirmData.status)
      startPolling()
    } catch (e) {
      setError(`Network error: ${e instanceof Error ? e.message : String(e)}`)
      setStep("error")
    }
  }

  const simulateWebhook = async () => {
    try {
      const res = await fetch(`${API}/webhooks/bank`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          bank_session_id: sessionID,
          status: "COMPLETED",
          timestamp: new Date().toISOString(),
          signature: "simulated",
        }),
      })
      if (!res.ok) {
        const data = await res.json()
        setError(data.error || "Bank webhook failed")
        setStep("error")
      }
    } catch (e) {
      setError(`Network error: ${e instanceof Error ? e.message : String(e)}`)
      setStep("error")
    }
  }

  const startPolling = useCallback(() => {
    if (pollingRef.current) return
    pollingRef.current = true
    let attempts = 0
    const maxAttempts = 20

    const poll = async () => {
      while (attempts < maxAttempts) {
        await new Promise((r) => setTimeout(r, 1500))
        attempts++
        try {
          const res = await fetch(`${API}/payments/status?bank_session_id=${sessionID}`)
          const data = await res.json()
          if (!res.ok) continue
          setStatus(data.status)
          if (data.status === "COMPLETED" || data.status === "FAILED" || data.status === "REFUNDED") {
            setStep("done")
            return
          }
        } catch {
          // ignore
        }
      }
      setError("Payment timeout — no response from bank")
      setStep("error")
    }

    void poll()
  }, [sessionID])

  const reset = () => {
    pollingRef.current = false
    setStep("loading")
    setError("")
    const session = new URLSearchParams(window.location.search).get("session")
    if (session) void loadPayment(session)
  }

  if (step === "loading") {
    return (
      <Card elevate width="100%" maxWidth={420} padding="$8" borderRadius="$6" backgroundColor="white">
        <YStack alignItems="center" space="$4">
          <Logo />
          <Spinner size="large" color="$blue10" />
          <Text color="gray" fontSize="$3">Loading payment details...</Text>
        </YStack>
      </Card>
    )
  }

  return (
    <Card elevate width="100%" maxWidth={420} padding="$6" borderRadius="$6" backgroundColor="white" shadowColor="rgba(0,0,0,0.3)" shadowRadius={20}>
      <YStack space="$5">
        <Logo />

        {details && (
          <YStack space="$2" backgroundColor="$gray2" padding="$3" borderRadius="$4">
            <XStack justifyContent="space-between">
              <Text color="gray" fontSize="$2">Order</Text>
              <Text fontWeight="600" fontSize="$3">{details.order_id}</Text>
            </XStack>
            <Separator />
            <XStack justifyContent="space-between">
              <Text color="gray" fontSize="$2">Amount</Text>
              <Text fontWeight="700" fontSize="$5" color="$blue10">
                {formatAmount(details.amount, details.currency)}
              </Text>
            </XStack>
          </YStack>
        )}

        {step === "3ds" && (
          <YStack space="$4">
            <Text fontSize="$5" fontWeight="600" textAlign="center">
              Secure Authentication
            </Text>
            <YStack
              borderWidth={2}
              borderStyle="dashed"
              borderColor="$blue8"
              borderRadius="$4"
              padding="$6"
              minHeight={260}
              alignItems="center"
              justifyContent="center"
              backgroundColor="$blue2"
              position="relative"
            >
              <XStack
                position="absolute"
                top={-12}
                backgroundColor="$blue8"
                paddingHorizontal="$3"
                paddingVertical="$1"
                borderRadius="$3"
              >
                <Text color="white" fontSize="$2" fontWeight="600">
                  3D Secure / ACS
                </Text>
              </XStack>

              <YStack alignItems="center" space="$3" marginTop="$4">
                <LockIcon />
                <Text fontSize="$3" color="gray" textAlign="center">
                  Bank authentication required
                </Text>
                <Text fontSize="$2" color="gray" textAlign="center">
                  In production, your bank's 3D Secure page loads here in a
                  secure iframe (https only, sandboxed).
                </Text>
                <Button
                  onPress={simulate3DS}
                  backgroundColor="$blue10"
                  color="white"
                  hoverStyle={{ backgroundColor: "$blue9" }}
                  pressStyle={{ backgroundColor: "$blue11" }}
                  marginTop="$2"
                >
                  Simulate 3DS Auth
                </Button>
                <Text fontSize="$1" color="gray" textAlign="center">
                  Development mode — simulated authentication
                </Text>
              </YStack>
            </YStack>
          </YStack>
        )}

        {step === "waiting" && (
          <YStack space="$4" alignItems="center" padding="$6">
            <Spinner size="large" color="$blue10" />
            <Text fontSize="$5" fontWeight="600">Processing payment...</Text>
            <XStack space="$2" alignItems="center">
              <Text fontSize="$3" color="gray">Status:</Text>
              <Text fontSize="$3" color="$blue10" fontWeight="600">{status}</Text>
            </XStack>
            <Button
              onPress={simulateWebhook}
              backgroundColor="$green10"
              color="white"
              hoverStyle={{ backgroundColor: "$green9" }}
              pressStyle={{ backgroundColor: "$green11" }}
              marginTop="$2"
            >
              Simulate Bank Webhook (COMPLETED)
            </Button>
            <Text fontSize="$1" color="gray" textAlign="center">
              Dev only — triggers bank completion callback
            </Text>
          </YStack>
        )}

        {step === "done" && details && (
          <YStack space="$4" alignItems="center" padding="$4">
            <StatusIcon status={status} />
            <Text
              fontSize="$8"
              fontWeight="700"
              color={status === "COMPLETED" ? "$green10" : status === "REFUNDED" ? "$orange10" : "$red10"}
            >
              {status === "COMPLETED" ? "Payment Successful" : status === "REFUNDED" ? "Payment Refunded" : "Payment Failed"}
            </Text>
            <YStack space="$2" width="100%" backgroundColor="$gray2" padding="$3" borderRadius="$4">
              <XStack justifyContent="space-between">
                <Text color="gray" fontSize="$2">Order</Text>
                <Text fontWeight="600" fontSize="$3">{details.order_id}</Text>
              </XStack>
              <XStack justifyContent="space-between">
                <Text color="gray" fontSize="$2">Amount</Text>
                <Text fontWeight="700">{formatAmount(details.amount, details.currency)}</Text>
              </XStack>
              <XStack justifyContent="space-between">
                <Text color="gray" fontSize="$2">Session</Text>
                <Text fontWeight="500" fontSize="$2" maxWidth={180} numberOfLines={1}>{sessionID}</Text>
              </XStack>
            </YStack>
            {status === "COMPLETED" && details.return_url && (
              <Button
                backgroundColor="$blue10"
                color="white"
                hoverStyle={{ backgroundColor: "$blue9" }}
                onPress={() => { window.location.href = details.return_url }}
              >
                Return to Merchant
              </Button>
            )}
            <Button
              variant="outlined"
              borderColor="$blue8"
              color="$blue10"
              onPress={reset}
            >
              New Payment
            </Button>
          </YStack>
        )}

        {step === "error" && (
          <YStack space="$4" alignItems="center" padding="$4">
            <Text fontSize="$6" fontWeight="700" color="$red10">Error</Text>
            <Text fontSize="$3" color="gray" textAlign="center">{error}</Text>
            <Button
              backgroundColor="$blue10"
              color="white"
              onPress={reset}
            >
              Try Again
            </Button>
          </YStack>
        )}

        <Footer />
      </YStack>
    </Card>
  )
}

function Logo() {
  return (
    <YStack alignItems="center" space="$2">
      <XStack
        width={56}
        height={56}
        borderRadius={14}
        backgroundColor="$blue10"
        alignItems="center"
        justifyContent="center"
      >
        <Text color="white" fontSize="$8" fontWeight="800">
          P
        </Text>
      </XStack>
      <Text fontSize="$5" fontWeight="700" color="$gray11" letterSpacing={2}>
        PAYGATE
      </Text>
    </YStack>
  )
}

function LockIcon() {
  return (
    <XStack
      width={48}
      height={48}
      borderRadius={24}
      backgroundColor="$green3"
      alignItems="center"
      justifyContent="center"
    >
      <Text fontSize="$7">🔒</Text>
    </XStack>
  )
}

function StatusIcon({ status }: { status: string }) {
  if (status === "COMPLETED") {
    return (
      <XStack
        width={64}
        height={64}
        borderRadius={32}
        backgroundColor="$green3"
        alignItems="center"
        justifyContent="center"
      >
        <Text fontSize="$9">✓</Text>
      </XStack>
    )
  }
  if (status === "REFUNDED") {
    return (
      <XStack
        width={64}
        height={64}
        borderRadius={32}
        backgroundColor="$orange3"
        alignItems="center"
        justifyContent="center"
      >
        <Text fontSize="$9">↩</Text>
      </XStack>
    )
  }
  return (
    <XStack
      width={64}
      height={64}
      borderRadius={32}
      backgroundColor="$red3"
      alignItems="center"
      justifyContent="center"
    >
      <Text fontSize="$9">✕</Text>
    </XStack>
  )
}

function Footer() {
  return (
    <YStack alignItems="center" marginTop="$2">
      <Text fontSize="$1" color="gray" textAlign="center">
        Secure payment powered by Paygate
      </Text>
      <Text fontSize="$1" color="gray" textAlign="center">
        PCI DSS compliant • 3D Secure
      </Text>
    </YStack>
  )
}

export default App

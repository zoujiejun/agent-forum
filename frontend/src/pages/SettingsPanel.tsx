import React from 'react'
import { Button, Card, Input, message, Space } from 'antd'
import { DEFAULT_AGENT_NAME, getAgentName, registerMember, setAgentName } from '../api'

export default function SettingsPanel({ onChanged }: { onChanged: () => void }) {
  const [name, setName] = React.useState(getAgentName())
  const save = () => {
    setAgentName(name.trim() || DEFAULT_AGENT_NAME)
    message.success('Identity saved')
    onChanged()
  }

  const register = async () => {
    try {
      await registerMember(name.trim() || DEFAULT_AGENT_NAME)
      message.success('Member registered successfully')
      onChanged()
    } catch (e: any) {
      message.error('Registration failed: ' + (e?.response?.data?.error || e?.message || e))
    }
  }

  return (
    <Card title="Identity Settings" size="small">
      <Space direction="vertical" style={{ width: '100%' }}>
        <Input value={name} onChange={e => setName(e.target.value)} placeholder={`Current identity, for example ${DEFAULT_AGENT_NAME}`} />
        <div style={{ display: 'flex', gap: 8 }}>
          <Button type="primary" onClick={save}>Save Identity</Button>
          <Button onClick={register}>Register Member</Button>
        </div>
      </Space>
    </Card>
  )
}

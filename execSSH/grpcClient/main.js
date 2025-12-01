const grpc = require('@grpc/grpc-js');
const protoLoader = require('@grpc/proto-loader');
const fs = require('fs');
const path = require('path');

const PROTO_PATH = path.join(__dirname, '..', 'protos', 'ssh.proto');
const ROOT_CERT_PATH = path.join(__dirname, '..', 'certificate/server.crt');

// Carregar proto
const packageDefinition = protoLoader.loadSync(PROTO_PATH, {
  keepCase: true,
  longs: String,
  enums: String,
  defaults: true,
  oneofs: true
});
const sshProto = grpc.loadPackageDefinition(packageDefinition).sshService;

// Ler certificado do servidor
const rootCert = fs.readFileSync(ROOT_CERT_PATH);
const creds = grpc.credentials.createSsl(
  rootCert,
  null,
  null,
  {
    checkServerIdentity: () => null // desativa a validação de hostname
  }
);

// Criar cliente gRPC TLS
const client = new sshProto.SSH('localhost:50051', creds);

function execSSH(command, host, port, user, password, privateKey) {
  return new Promise((resolve, reject) => {
    const request = {
      command,
      host,
      port,
      user,
      password: password || '',
      privateKey: privateKey || ''
    };

    client.ExecCommand(request, (err, response) => {
      if (err) return reject(err);
      resolve(response);
    });
  });
}

// Exemplo de uso
(async () => {
  try {
    const res = await execSSH(
      'ls -la',
      '212.85.1.223',
      22,
      'other',
      '42luanda',
      ''
    );

    console.log('STDOUT:\n', res.stdout);
    console.log('STDERR:\n', res.stderr);
    console.log('Exit code:', res.exitCode);
  } catch (err) {
    console.error('Erro:', err);
  }
})();

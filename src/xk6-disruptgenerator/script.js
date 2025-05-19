import grpc from 'src/xk6-disruptgenerator/k6/x/grpc';
import { sleep } from 'src/xk6-disruptgenerator/k6';

const client = new grpc.Client();

client.load(null, './proto/currencyservice/demo.proto');

export default () => {
    client.connect('currencyservice:7000', { plaintext: true });

    const response =  client.invoke('hipstershop.CurrencyService/GetSupportedCurrencies', {});

    console.log("response: ", JSON.stringify(response.message));

    client.close();

    sleep(1);
};
